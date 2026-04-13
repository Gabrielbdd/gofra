export type AuthConfig = {
  issuer: string;
  clientId: string;
  scopes: string[];
  redirectPath: string;
  postLogoutRedirectPath: string;
};

export type RuntimeConfig = {
  apiBaseUrl: string;
  auth: AuthConfig;
};
