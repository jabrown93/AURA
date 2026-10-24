import axios from "axios";

const apiClient = axios.create({
  baseURL: "/api",
  timeout: 3000000,
  withCredentials: true, // send/receive the aura_session HttpOnly cookie
  headers: {
    "Content-Type": "application/json",
  },
});

// In setup mode without a login, every request needs the setup code, stored for this browser session
export const SETUP_CODE_STORAGE_KEY = "auraSetupCode";
export const SETUP_CODE_REQUIRED_EVENT = "aura:setup-code-required";

apiClient.interceptors.request.use((request) => {
  if (typeof window !== "undefined") {
    try {
      const setupCode = sessionStorage.getItem(SETUP_CODE_STORAGE_KEY);
      if (setupCode) request.headers.set("X-Setup-Code", setupCode);
    } catch {
      // Without sessionStorage, the setup code is asked for again
    }
  }
  return request;
});

// Auto redirect on 401 - session cookie missing/expired
// On 403 with setup_code_required, ask for the setup code again
apiClient.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err?.response?.status === 401 && typeof window !== "undefined") {
      if (!window.location.pathname.startsWith("/login")) {
        window.location.href = "/login";
        return new Promise(() => {});
      }
    }
    if (err?.response?.status === 403 && err?.response?.data?.setup_code_required && typeof window !== "undefined") {
      try {
        sessionStorage.removeItem(SETUP_CODE_STORAGE_KEY);
      } catch {
        // Nothing stored
      }
      window.dispatchEvent(new Event(SETUP_CODE_REQUIRED_EVENT));
    }
    return Promise.reject(err);
  }
);

export default apiClient;
