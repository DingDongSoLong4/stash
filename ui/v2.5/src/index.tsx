import "src/polyfills";

import { ApolloProvider } from "@apollo/client";
import ReactDOM from "react-dom";
import { BrowserRouter } from "react-router-dom";
import { App } from "./App";
import { getClient } from "./core/StashService";
import { baseURL, getPlatformURL } from "./core/createClient";
import "./index.scss";

ReactDOM.render(
  <>
    <link
      rel="stylesheet"
      type="text/css"
      href={getPlatformURL("css").toString()}
    />
    <BrowserRouter basename={baseURL}>
      <ApolloProvider client={getClient()}>
        <App />
      </ApolloProvider>
    </BrowserRouter>
  </>,
  document.getElementById("root")
);

const script = document.createElement("script");
script.src = getPlatformURL("javascript").toString();
document.body.appendChild(script);
