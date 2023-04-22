import React from "react";
import { IConfig } from "src/core/config";

export interface IContext {
  configuration?: IConfig;
  loading?: boolean;
}

export const ConfigurationContext = React.createContext<IContext>({});

export const ConfigurationProvider: React.FC<IContext> = ({
  loading,
  configuration,
  children,
}) => {
  return (
    <ConfigurationContext.Provider
      value={{
        configuration,
        loading,
      }}
    >
      {children}
    </ConfigurationContext.Provider>
  );
};
