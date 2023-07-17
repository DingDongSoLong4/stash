package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	gqlHandler "github.com/99designs/gqlgen/graphql/handler"
	gqlExtension "github.com/99designs/gqlgen/graphql/handler/extension"
	gqlLru "github.com/99designs/gqlgen/graphql/handler/lru"
	gqlTransport "github.com/99designs/gqlgen/graphql/handler/transport"
	gqlPlayground "github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog"
	"github.com/gorilla/websocket"
	"github.com/vearutop/statigz"

	"github.com/stashapp/stash/internal/api/loaders"
	"github.com/stashapp/stash/internal/build"
	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/fsutil"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/plugin"
	"github.com/stashapp/stash/pkg/utils"
	"github.com/stashapp/stash/ui"
)

const (
	loginEndpoint      = "/login"
	logoutEndpoint     = "/logout"
	gqlEndpoint        = "/graphql"
	playgroundEndpoint = "/playground"
)

type Server struct {
	http.Server
	displayAddress string

	manager *manager.Manager
}

func Initialize() (*Server, error) {
	initialiseImages()

	mgr := manager.GetInstance()
	c := mgr.Config

	displayHost := c.GetHost()
	if displayHost == "0.0.0.0" {
		displayHost = "localhost"
	}
	displayAddress := displayHost + ":" + strconv.Itoa(c.GetPort())

	address := c.GetHost() + ":" + strconv.Itoa(c.GetPort())
	tlsConfig, err := makeTLSConfig(c)
	if err != nil {
		// assume we don't want to start with a broken TLS configuration
		return nil, fmt.Errorf("error loading TLS config: %v", err)
	}

	if tlsConfig != nil {
		displayAddress = "https://" + displayAddress + "/"
	} else {
		displayAddress = "http://" + displayAddress + "/"
	}

	r := chi.NewRouter()

	server := &Server{
		Server: http.Server{
			Addr:      address,
			Handler:   r,
			TLSConfig: tlsConfig,
			// disable http/2 support by default
			// when http/2 is enabled, we are unable to hijack and close
			// the connection/request. This is necessary to stop running
			// streams when deleting a scene file.
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		},
		displayAddress: displayAddress,
		manager:        mgr,
	}

	r.Use(middleware.Heartbeat("/healthz"))
	r.Use(cors.AllowAll().Handler)
	r.Use(authenticateHandler())
	visitedPluginHandler := mgr.SessionStore.VisitedPluginHandler()
	r.Use(visitedPluginHandler)

	r.Use(middleware.Recoverer)

	if c.GetLogAccess() {
		httpLogger := httplog.NewLogger("Stash", httplog.Options{
			Concise: true,
		})
		r.Use(httplog.RequestLogger(httpLogger))
	}
	r.Use(SecurityHeadersMiddleware)
	r.Use(middleware.DefaultCompress)
	r.Use(middleware.StripSlashes)
	r.Use(BaseURLMiddleware)

	recoverFunc := func(ctx context.Context, err interface{}) error {
		logger.Error(err)
		debug.PrintStack()

		message := fmt.Sprintf("Internal system error. Error <%v>", err)
		return errors.New(message)
	}

	repo := mgr.Repository

	dataloaders := loaders.Middleware{
		Repository: repo,
	}

	r.Use(dataloaders.Middleware)

	pluginCache := mgr.PluginCache
	sceneService := mgr.SceneService
	imageService := mgr.ImageService
	galleryService := mgr.GalleryService
	resolver := &Resolver{
		repository:     repo,
		sceneService:   sceneService,
		imageService:   imageService,
		galleryService: galleryService,
		hookExecutor:   pluginCache,
	}

	gqlSrv := gqlHandler.New(NewExecutableSchema(Config{Resolvers: resolver}))
	gqlSrv.SetRecoverFunc(recoverFunc)
	gqlSrv.AddTransport(gqlTransport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		KeepAlivePingInterval: 10 * time.Second,
	})
	gqlSrv.AddTransport(gqlTransport.Options{})
	gqlSrv.AddTransport(gqlTransport.GET{})
	gqlSrv.AddTransport(gqlTransport.POST{})
	gqlSrv.AddTransport(gqlTransport.MultipartForm{
		MaxUploadSize: c.GetMaxUploadSize(),
	})

	gqlSrv.SetQueryCache(gqlLru.New(1000))
	gqlSrv.Use(gqlExtension.Introspection{})

	gqlSrv.SetErrorPresenter(gqlErrorHandler)

	gqlHandlerFunc := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		gqlSrv.ServeHTTP(w, r)
	}

	// register GQL handler with plugin cache
	// chain the visited plugin handler
	// also requires the dataloader middleware
	gqlHandler := visitedPluginHandler(dataloaders.Middleware(http.HandlerFunc(gqlHandlerFunc)))
	pluginCache.RegisterGQLHandler(gqlHandler)

	r.HandleFunc(gqlEndpoint, gqlHandlerFunc)
	r.HandleFunc(playgroundEndpoint, func(w http.ResponseWriter, r *http.Request) {
		setPageSecurityHeaders(w, r)
		endpoint := getProxyPrefix(r) + gqlEndpoint
		gqlPlayground.Handler("GraphQL playground", endpoint)(w, r)
	})

	r.Mount("/performer", server.getPerformerRoutes())
	r.Mount("/scene", server.getSceneRoutes())
	r.Mount("/image", server.getImageRoutes())
	r.Mount("/studio", server.getStudioRoutes())
	r.Mount("/movie", server.getMovieRoutes())
	r.Mount("/tag", server.getTagRoutes())
	r.Mount("/downloads", server.getDownloadsRoutes())

	r.HandleFunc("/css", cssHandler(c, pluginCache))
	r.HandleFunc("/javascript", javascriptHandler(c, pluginCache))
	r.HandleFunc("/customlocales", customLocalesHandler(c))

	staticLoginUI := statigz.FileServer(ui.LoginUIBox.(fs.ReadDirFS))

	r.Get(loginEndpoint, handleLogin(ui.LoginUIBox))
	r.Post(loginEndpoint, handleLoginPost(ui.LoginUIBox))
	r.Get(logoutEndpoint, handleLogout())
	r.HandleFunc(loginEndpoint+"/*", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, loginEndpoint)
		w.Header().Set("Cache-Control", "no-cache")
		staticLoginUI.ServeHTTP(w, r)
	})

	// Serve static folders
	customServedFolders := c.GetCustomServedFolders()
	if customServedFolders != nil {
		r.Mount("/custom", getCustomRoutes(customServedFolders))
	}

	customUILocation := c.GetCustomUILocation()
	staticUI := statigz.FileServer(ui.UIBox.(fs.ReadDirFS))

	// Serve the web app
	r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		ext := path.Ext(r.URL.Path)

		if customUILocation != "" {
			if r.URL.Path == "index.html" || ext == "" {
				r.URL.Path = "/"
			}

			http.FileServer(http.Dir(customUILocation)).ServeHTTP(w, r)
			return
		}

		if ext == ".html" || ext == "" {
			themeColor := c.GetThemeColor()
			data, err := fs.ReadFile(ui.UIBox, "index.html")
			if err != nil {
				panic(err)
			}
			indexHtml := string(data)

			prefix := getProxyPrefix(r)
			indexHtml = strings.ReplaceAll(indexHtml, "%COLOR%", themeColor)
			indexHtml = strings.Replace(indexHtml, `<base href="/"`, fmt.Sprintf(`<base href="%s/"`, prefix), 1)

			w.Header().Set("Content-Type", "text/html")
			setPageSecurityHeaders(w, r)

			utils.ServeStaticContent(w, r, []byte(indexHtml))
		} else {
			isStatic, _ := path.Match("/assets/*", r.URL.Path)
			if isStatic {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}

			staticUI.ServeHTTP(w, r)
		}
	})

	logger.Infof("stash version: %s\n", build.VersionString())
	go printLatestVersion(context.TODO())

	return server, nil
}

func (s *Server) Start() error {
	logger.Infof("stash is listening on " + s.Addr)
	logger.Infof("stash is running at " + s.displayAddress)

	if s.TLSConfig != nil {
		return s.ListenAndServeTLS("", "")
	} else {
		return s.ListenAndServe()
	}
}

func (s *Server) getPerformerRoutes() chi.Router {
	repo := s.manager.Repository
	return performerRoutes{
		routes:    routes{txnManager: repo.Database},
		performer: repo.Performer,
	}.Routes()
}

func (s *Server) getSceneRoutes() chi.Router {
	repo := s.manager.Repository
	return sceneRoutes{
		routes:      routes{txnManager: repo.Database},
		scene:       repo.Scene,
		file:        repo.File,
		sceneMarker: repo.SceneMarker,
		tag:         repo.Tag,
	}.Routes()
}

func (s *Server) getImageRoutes() chi.Router {
	repo := s.manager.Repository
	return imageRoutes{
		routes: routes{txnManager: repo.Database},
		image:  repo.Image,
		file:   repo.File,
	}.Routes()
}

func (s *Server) getStudioRoutes() chi.Router {
	repo := s.manager.Repository
	return studioRoutes{
		routes: routes{txnManager: repo.Database},
		studio: repo.Studio,
	}.Routes()
}

func (s *Server) getMovieRoutes() chi.Router {
	repo := s.manager.Repository
	return movieRoutes{
		routes: routes{txnManager: repo.Database},
		movie:  repo.Movie,
	}.Routes()
}

func (s *Server) getTagRoutes() chi.Router {
	repo := s.manager.Repository
	return tagRoutes{
		routes: routes{txnManager: repo.Database},
		tag:    repo.Tag,
	}.Routes()
}

func (s *Server) getDownloadsRoutes() chi.Router {
	return downloadsRoutes{}.Routes()
}

func copyFile(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(w, f)

	return err
}

func serveFiles(w http.ResponseWriter, r *http.Request, paths []string) {
	buffer := bytes.Buffer{}

	for _, path := range paths {
		err := copyFile(&buffer, path)
		if err != nil {
			logger.Errorf("error serving file %s: %v", path, err)
		}
		buffer.Write([]byte("\n"))
	}

	utils.ServeStaticContent(w, r, buffer.Bytes())
}

func cssHandler(c *config.Instance, pluginCache *plugin.Cache) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// add plugin css files first
		var paths []string

		for _, p := range pluginCache.ListPlugins() {
			paths = append(paths, p.UI.CSS...)
		}

		if c.GetCSSEnabled() {
			// search for custom.css in current directory, then $HOME/.stash
			fn := c.GetCSSPath()
			exists, _ := fsutil.FileExists(fn)
			if exists {
				paths = append(paths, fn)
			}
		}

		w.Header().Set("Content-Type", "text/css")
		serveFiles(w, r, paths)
	}
}

func javascriptHandler(c *config.Instance, pluginCache *plugin.Cache) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// add plugin javascript files first
		var paths []string

		for _, p := range pluginCache.ListPlugins() {
			paths = append(paths, p.UI.Javascript...)
		}

		if c.GetJavascriptEnabled() {
			// search for custom.js in current directory, then $HOME/.stash
			fn := c.GetJavascriptPath()
			exists, _ := fsutil.FileExists(fn)
			if exists {
				paths = append(paths, fn)
			}
		}

		w.Header().Set("Content-Type", "text/javascript")
		serveFiles(w, r, paths)
	}
}

func customLocalesHandler(c *config.Instance) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.Buffer{}

		if c.GetCustomLocalesEnabled() {
			// search for custom-locales.json in current directory, then $HOME/.stash
			path := c.GetCustomLocalesPath()
			exists, _ := fsutil.FileExists(path)
			if exists {
				err := copyFile(&buffer, path)
				if err != nil {
					logger.Errorf("error serving file %s: %v", path, err)
				}
			}
		}

		if buffer.Len() == 0 {
			buffer.Write([]byte("{}"))
		}

		w.Header().Set("Content-Type", "application/json")
		utils.ServeStaticContent(w, r, buffer.Bytes())
	}
}

func makeTLSConfig(c *config.Instance) (*tls.Config, error) {
	c.InitTLS()
	certFile, keyFile := c.GetTLSFiles()

	if certFile == "" && keyFile == "" {
		// assume http configuration
		return nil, nil
	}

	// ensure both files are present
	if certFile == "" {
		return nil, errors.New("SSL certificate file must be present if key file is present")
	}

	if keyFile == "" {
		return nil, errors.New("SSL key file must be present if certificate file is present")
	}

	cert, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("error reading SSL certificate file %s: %v", certFile, err)
	}

	key, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("error reading SSL key file %s: %v", keyFile, err)
	}

	certs := make([]tls.Certificate, 1)
	certs[0], err = tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("error parsing key pair: %v", err)
	}
	tlsConfig := &tls.Config{
		Certificates: certs,
	}

	return tlsConfig, nil
}

func setPageSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	c := config.GetInstance()

	defaultSrc := "data: 'self' 'unsafe-inline'"
	connectSrc := "data: 'self'"
	imageSrc := "data: *"
	scriptSrc := "'self' http://www.gstatic.com https://www.gstatic.com 'unsafe-inline' 'unsafe-eval'"
	styleSrc := "'self' 'unsafe-inline'"
	mediaSrc := "blob: 'self'"

	// Workaround Safari bug https://bugs.webkit.org/show_bug.cgi?id=201591
	// Allows websocket requests to any origin
	connectSrc += " ws: wss:"

	// The graphql playground pulls its frontend from a cdn
	if r.URL.Path == playgroundEndpoint {
		connectSrc += " https://cdn.jsdelivr.net"
		scriptSrc += " https://cdn.jsdelivr.net"
		styleSrc += " https://cdn.jsdelivr.net"
	}

	if !c.IsNewSystem() && c.GetHandyKey() != "" {
		connectSrc += " https://www.handyfeeling.com"
	}

	cspDirectives := fmt.Sprintf("default-src %s; connect-src %s; img-src %s; script-src %s; style-src %s; media-src %s;", defaultSrc, connectSrc, imageSrc, scriptSrc, styleSrc, mediaSrc)
	cspDirectives += " worker-src blob:; child-src 'none'; object-src 'none'; form-action 'self';"

	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", cspDirectives)
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

type contextKey struct {
	name string
}

var (
	BaseURLCtxKey = &contextKey{"BaseURL"}
)

func BaseURLMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		scheme := "http"
		if strings.Compare("https", r.URL.Scheme) == 0 || r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		prefix := getProxyPrefix(r)

		baseURL := scheme + "://" + r.Host + prefix

		externalHost := config.GetInstance().GetExternalHost()
		if externalHost != "" {
			baseURL = externalHost + prefix
		}

		r = r.WithContext(context.WithValue(ctx, BaseURLCtxKey, baseURL))

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func getProxyPrefix(r *http.Request) string {
	return strings.TrimRight(r.Header.Get("X-Forwarded-Prefix"), "/")
}
