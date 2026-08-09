import type { ReactNode } from 'react';

import { createContext, use, useCallback, useEffect, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { toast } from 'sonner';

import type { AuthInfo } from '@/models/info';

import { api, getApiErrorStatusCode } from '@/lib/axios';
import { getReturnUrlParam } from '@/lib/utils/auth';
import { baseUrl } from '@/models/api';

export interface LoginCredentials {
    mail: string;
    password: string;
}

export interface LoginResult {
    error?: string;
    passwordChangeRequired?: boolean;
    success: boolean;
}

export type OAuthProvider = 'github' | 'google';

interface UserContextType {
    authInfo: AuthInfo | null;
    // True while the backend is unreachable (pod resuming/starting, 5xx from the
    // gateway, or a network error). The app shows a "starting up" screen and
    // keeps polling /info instead of falling through to the login page.
    backendUnreachable: boolean;
    clearAuth: () => void;
    isAuthenticated: () => boolean;
    isLoading: boolean;
    login: (credentials: LoginCredentials) => Promise<LoginResult>;
    loginWithOAuth: (provider: OAuthProvider) => Promise<LoginResult>;
    logout: (returnUrl?: string) => Promise<void>;
    refreshAuthInfo: () => Promise<void>;
    setAuth: (authInfo: AuthInfo) => void;
}

const UserContext = createContext<undefined | UserContextType>(undefined);

export const AUTH_STORAGE_KEY = 'auth';

export function UserProvider({ children }: { children: ReactNode }) {
    const navigate = useNavigate();
    const location = useLocation();
    const [authInfo, setAuthInfo] = useState<AuthInfo | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const [backendUnreachable, setBackendUnreachable] = useState(false);

    useEffect(() => {
        let retryTimer: ReturnType<typeof setTimeout> | undefined;
        let cancelled = false;

        // Fetch /info, retrying while the backend is unreachable. This deployment
        // runs behind an SSO gateway with seamless no-login (AUTH_AUTO_LOGIN), so a
        // failing /info almost always means the pod is resuming/starting — showing
        // the login page there is wrong (there are no credentials). We keep polling
        // and surface a "starting up" screen until the server answers.
        const attemptInfo = async () => {
            try {
                const info = await api.get<AuthInfo>('/info');

                if (cancelled) {
                    return;
                }

                // Seamless no-login (AUTH_AUTO_LOGIN) deployment still cold-starting:
                // the server answers but returns a guest because the admin session
                // isn't populated yet (fresh-DB migrations seed the admin a beat after
                // the HTTP server accepts traffic). Keep polling and show the "starting
                // up" screen — it flips to the admin user once startup completes, so we
                // never bounce the user to a login page they have no credentials for.
                if (info?.status === 'success' && info.data?.type === 'guest' && info.data.auto_login) {
                    setBackendUnreachable(true);
                    setIsLoading(false);
                    retryTimer = setTimeout(attemptInfo, 3000);

                    return;
                }

                if (info?.status === 'success' && info.data) {
                    setAuthInfo(info.data);

                    if (info.data.type === 'guest') {
                        localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(info.data));
                    }
                }

                setBackendUnreachable(false);
                setIsLoading(false);
            } catch (err) {
                if (cancelled) {
                    return;
                }

                const status = getApiErrorStatusCode(err);
                // undefined = network error; >=500 = gateway/router down; 404 = pod
                // mid-cold-start before its routes are mounted. All mean "not ready
                // yet" on this deployment, so keep polling behind the startup screen.
                const transportDown = status === undefined || status >= 500 || status === 404;

                if (transportDown) {
                    // Backend down/resuming — do not fall through to /login; keep polling.
                    setBackendUnreachable(true);
                    setIsLoading(false);
                    retryTimer = setTimeout(attemptInfo, 3000);
                } else {
                    // A definitive response (e.g. 200 unauthenticated). Let the normal
                    // guard decide (login page for non-auto-login deployments).
                    setBackendUnreachable(false);
                    setIsLoading(false);
                }
            }
        };

        const initializeAuth = async () => {
            try {
                const storedData = localStorage.getItem(AUTH_STORAGE_KEY);

                if (storedData) {
                    const parsedAuthInfo: AuthInfo = JSON.parse(storedData);

                    if (parsedAuthInfo) {
                        setAuthInfo(parsedAuthInfo);

                        // Cached users can render immediately, but must still probe
                        // /info. Otherwise a cold-start failure in the route refresh
                        // leaves the startup screen stuck until navigation happens.
                        if (parsedAuthInfo.type !== 'guest') {
                            setIsLoading(false);
                        }
                    }
                }
            } catch {
                localStorage.removeItem(AUTH_STORAGE_KEY);
            }

            await attemptInfo();
        };

        initializeAuth();

        return () => {
            cancelled = true;

            if (retryTimer) {
                clearTimeout(retryTimer);
            }
        };
    }, []);

    const setAuth = useCallback((newAuthInfo: AuthInfo) => {
        setAuthInfo(newAuthInfo);
        localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(newAuthInfo));
    }, []);

    const clearAuth = useCallback(() => {
        setAuthInfo(null);
        localStorage.removeItem(AUTH_STORAGE_KEY);
    }, []);

    const isAuthenticated = useCallback(() => {
        if (!authInfo?.user || !authInfo?.expires_at) {
            return false;
        }

        const now = new Date();
        const expirationDate = new Date(authInfo.expires_at);

        return expirationDate > now;
    }, [authInfo]);

    const refreshAuthInfo = useCallback(async () => {
        try {
            const info = await api.get<AuthInfo>('/info');

            if (info?.status === 'success' && info.data) {
                setAuth(info.data);
            } else {
                clearAuth();
            }
        } catch {
            clearAuth();
        }
    }, [setAuth, clearAuth]);

    useEffect(() => {
        if (location.pathname === '/login' && !isLoading) {
            // eslint-disable-next-line react-hooks/set-state-in-effect -- refreshAuthInfo's setState runs after an async fetch, not synchronously
            refreshAuthInfo();
        }
    }, [location.pathname, isLoading, refreshAuthInfo]);

    const logout = useCallback(
        async (returnUrl?: string) => {
            const currentPath = location.pathname;
            const finalReturnUrl = returnUrl || getReturnUrlParam(currentPath);

            try {
                await api.get('/auth/logout');
                toast.success('Successfully logged out');
            } catch {
                toast.error('Logout failed, but clearing local session');
            } finally {
                clearAuth();
                window.location.href = `/login${finalReturnUrl}`;
            }
        },
        [clearAuth, location.pathname],
    );

    const login = useCallback(
        async (credentials: LoginCredentials): Promise<LoginResult> => {
            try {
                const loginResponse = await api.post<unknown>('/auth/login', credentials);

                if (loginResponse?.status !== 'success') {
                    const errorMessage = 'Invalid login or password';
                    toast.error(errorMessage);

                    return { error: errorMessage, success: false };
                }

                // Backend sets the session cookie on /auth/login — fetch /info to materialize the user.
                const infoResponse = await api.get<AuthInfo>('/info');

                if (infoResponse?.status !== 'success' || !infoResponse.data) {
                    const errorMessage = 'Failed to load user information';
                    toast.error(errorMessage);

                    return { error: errorMessage, success: false };
                }

                setAuth(infoResponse.data);

                if (infoResponse.data.user?.type === 'local' && infoResponse.data.user.password_change_required) {
                    toast.warning('Password change required');

                    return { passwordChangeRequired: true, success: true };
                }

                return { success: true };
            } catch {
                const errorMessage = 'Login failed. Please try again.';
                toast.error(errorMessage);

                return { error: errorMessage, success: false };
            }
        },
        [setAuth],
    );

    const loginWithOAuth = useCallback(
        async (provider: OAuthProvider): Promise<LoginResult> => {
            const returnOAuthUri = '/oauth/result';
            const width = 500;
            const height = 600;
            const left = window.screenX + (window.outerWidth - width) / 2;
            const top = window.screenY + (window.outerHeight - height) / 2;

            const popup = window.open(
                `${baseUrl}/auth/authorize?provider=${provider}&return_uri=${returnOAuthUri}`,
                `${provider} Sign In`,
                `width=${width},height=${height},left=${left},top=${top}`,
            );

            if (!popup) {
                const errorMessage = 'Popup blocked. Please allow popups for this site.';
                toast.error(errorMessage);

                return {
                    error: errorMessage,
                    success: false,
                };
            }

            return new Promise<LoginResult>((resolve) => {
                const popupCheckInterval = 500;
                const popupTimeout = 300000;
                let isResolved = false;

                const popupCheck = setInterval(() => {
                    if (popup?.closed && !isResolved) {
                        isResolved = true;
                        clearInterval(popupCheck);
                        clearTimeout(timeoutId);
                        window.removeEventListener('message', messageHandler);
                        const errorMessage = 'Authentication cancelled';
                        toast.info(errorMessage);
                        resolve({
                            error: errorMessage,
                            success: false,
                        });
                    }
                }, popupCheckInterval);

                const timeoutId = setTimeout(() => {
                    if (!isResolved) {
                        isResolved = true;
                        clearInterval(popupCheck);
                        window.removeEventListener('message', messageHandler);

                        if (popup && !popup.closed) {
                            popup.close();
                        }

                        const errorMessage = 'Authentication timeout';
                        toast.error(errorMessage);
                        resolve({
                            error: errorMessage,
                            success: false,
                        });
                    }
                }, popupTimeout);

                const messageHandler = async (event: MessageEvent) => {
                    if (event.origin !== window.location.origin || event.data?.type !== 'oauth-result') {
                        return;
                    }

                    if (isResolved) {
                        return;
                    }

                    isResolved = true;
                    clearInterval(popupCheck);
                    clearTimeout(timeoutId);
                    window.removeEventListener('message', messageHandler);

                    const cleanup = () => {
                        if (popup && !popup.closed) {
                            popup.close();
                        }
                    };

                    if (event.data.status === 'success') {
                        try {
                            const info = await api.get<AuthInfo>('/info');

                            if (info?.status === 'success' && info.data?.type === 'user') {
                                setAuth(info.data);
                                cleanup();
                                resolve({ success: true });

                                return;
                            }
                        } catch (error) {
                            console.error('Error during OAuth result handling:', error);
                        }
                    }

                    cleanup();
                    const errorMessage = event.data.error || 'Authentication failed';
                    toast.error(errorMessage);
                    resolve({
                        error: errorMessage,
                        success: false,
                    });
                };

                window.addEventListener('message', messageHandler);
            });
        },
        [setAuth],
    );

    useEffect(() => {
        const updateAuth = async () => {
            const publicRoutes = ['/login', '/oauth/result'];

            if (publicRoutes.includes(location.pathname)) {
                return;
            }

            if (!isAuthenticated()) {
                return;
            }

            try {
                const info = await api.get<AuthInfo>('/info', {
                    params: {
                        refresh_cookie: true,
                    },
                });

                if (info?.status === 'success' && info.data) {
                    setBackendUnreachable(false);
                    setAuth(info.data);
                } else {
                    clearAuth();
                    toast.error('Session expired. Please login again.');
                    const returnParam = getReturnUrlParam(location.pathname);
                    navigate(`/login${returnParam}`);
                }
            } catch (err) {
                const status = getApiErrorStatusCode(err);

                // Backend momentarily unreachable (pod resuming, 5xx, network): keep
                // the existing session and let the app recover — do not force logout.
                if (status === undefined || status >= 500) {
                    setBackendUnreachable(true);

                    return;
                }

                clearAuth();
                toast.error('Session expired. Please login again.');
                const returnParam = getReturnUrlParam(location.pathname);
                navigate(`/login${returnParam}`);
            }
        };

        updateAuth();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [location.pathname]);

    useEffect(() => {
        const handleAuthRefresh = () => {
            refreshAuthInfo();
        };

        window.addEventListener('auth:refresh', handleAuthRefresh);

        return () => {
            window.removeEventListener('auth:refresh', handleAuthRefresh);
        };
    }, [refreshAuthInfo]);

    return (
        <UserContext
            value={{
                authInfo,
                backendUnreachable,
                clearAuth,
                isAuthenticated,
                isLoading,
                login,
                loginWithOAuth,
                logout,
                refreshAuthInfo,
                setAuth,
            }}
        >
            {children}
        </UserContext>
    );
}

export function useUser() {
    const context = use(UserContext);

    if (context === undefined) {
        throw new Error('useUser must be used within a UserProvider');
    }

    return context;
}
