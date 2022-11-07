import React, { createContext, useContext, useMemo, useState } from "react";
import Mousetrap from "mousetrap";
import "mousetrap-pause";

type Hotkeys = string | string[];
type HotkeyCallback = (
  e: Mousetrap.ExtendedKeyboardEvent,
  combo: string
) => boolean | void;

export interface IHotkeysContext {
  bind: (hotkeys: Hotkeys, callback: HotkeyCallback) => () => void;
  unbind: (hotkeys: Hotkeys, callback: HotkeyCallback) => void;
  pause: (combo?: string) => void;
  unpause: (combo?: string) => void;
}

export interface IHotkeys {
  context: IHotkeysContext;
  bind: (hotkeys: Hotkeys, callback: HotkeyCallback) => () => void;
  unbind: (hotkeys: Hotkeys) => void;
}

const HotkeysContext = createContext<IHotkeysContext>({
  bind: () => () => {},
  unbind: () => {},
  pause: () => {},
  unpause: () => {},
});

export const useHotkeysContext = () => {
  return useContext(HotkeysContext);
};

export const HotkeysProvider: React.FC = ({ children }) => {
  const [boundKeys] = useState<Map<string, HotkeyCallback[]>>(new Map());
  const [mousetrap] = useState(new Mousetrap());

  const contextValue = useMemo(() => {
    function hotkeyCallback(e: Mousetrap.ExtendedKeyboardEvent, combo: string) {
      let callbacks = boundKeys.get(combo);
      if (!callbacks) return;

      callbacks[0](e, combo);
    }

    function bindSingle(hotkey: string, callback: HotkeyCallback) {
      let callbacks = boundKeys.get(hotkey);
      if (callbacks) {
        callbacks.unshift(callback);
      } else {
        boundKeys.set(hotkey, [callback]);
      }
      mousetrap.bind(hotkey, hotkeyCallback);
    }

    function bind(hotkeys: Hotkeys, callback: HotkeyCallback) {
      if (Array.isArray(hotkeys)) {
        for (const hotkey of hotkeys) {
          bindSingle(hotkey, callback);
        }
      } else {
        bindSingle(hotkeys, callback);
      }
      return unbind.bind(undefined, hotkeys, callback);
    }

    function unbindSingle(hotkey: string, callback: HotkeyCallback) {
      const callbacks = boundKeys.get(hotkey);
      if (callbacks) {
        for (let i = 0; i < callbacks.length; i++) {
          if (callback === callbacks[i]) {
            callbacks.splice(i, 1);
            break;
          }
        }
        if (callbacks.length === 0) {
          mousetrap.unbind(hotkey);
          boundKeys.delete(hotkey);
        }
      }
    }

    function unbind(hotkeys: Hotkeys, callback: HotkeyCallback) {
      if (Array.isArray(hotkeys)) {
        for (const hotkey of hotkeys) {
          unbindSingle(hotkey, callback);
        }
      } else {
        unbindSingle(hotkeys, callback);
      }
    }

    function pause(combo?: string) {
      if (combo) {
        mousetrap.pauseCombo(combo);
      } else {
        mousetrap.pause();
      }
    }

    function unpause(combo?: string) {
      if (combo) {
        mousetrap.unpauseCombo(combo);
      } else {
        mousetrap.unpause();
      }
    }

    return {
      bind,
      unbind,
      pause,
      unpause,
    };
  }, [boundKeys, mousetrap]);

  return (
    <HotkeysContext.Provider value={contextValue}>
      {children}
    </HotkeysContext.Provider>
  );
};

export const useHotkeys: () => IHotkeys = () => {
  const context = useHotkeysContext();
  const [boundKeys] = useState<Map<Hotkeys, HotkeyCallback>>(new Map());

  return useMemo(() => {
    function bind(hotkeys: Hotkeys, callback: HotkeyCallback) {
      const oldCallback = boundKeys.get(hotkeys);
      if (oldCallback) {
        context.unbind(hotkeys, oldCallback);
      }
      context.bind(hotkeys, callback);
      boundKeys.set(hotkeys, callback);
      return unbind.bind(undefined, hotkeys, callback);
    }

    function unbind(hotkeys: Hotkeys) {
      const oldCallback = boundKeys.get(hotkeys);
      if (oldCallback) {
        context.unbind(hotkeys, oldCallback);
        boundKeys.delete(hotkeys);
      }
    }

    return {
      context,
      bind,
      unbind,
    };
  }, [context, boundKeys]);
};
