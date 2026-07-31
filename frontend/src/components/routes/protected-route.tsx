import { Loader2 } from 'lucide-react';
import * as React from 'react';
import { Navigate, useLocation } from 'react-router-dom';

import { getReturnUrlParam } from '@/lib/utils/auth';
import { useUser } from '@/providers/user-provider';

// Optional auto-login for single-tenant deployments sitting behind an external SSO gateway.
// When both build-time env vars are present, the app silently signs in using them and the
// internal login page is effectively hidden. Credentials are read once at module load; the
// password is never logged.
const AUTOLOGIN_EMAIL = import.meta.env.VITE_AUTOLOGIN_EMAIL;
const AUTOLOGIN_PASSWORD = import.meta.env.VITE_AUTOLOGIN_PASSWORD;
const AUTOLOGIN_ENABLED = Boolean(AUTOLOGIN_EMAIL && AUTOLOGIN_PASSWORD);

// Module-level guard so the silent login is attempted at most once per page-load session,
// even as ProtectedRoute mounts/unmounts across navigations.
let autoLoginAttempted = false;

function ProtectedRoute({ children }: { children: React.ReactNode }) {
    const location = useLocation();
    const { isAuthenticated, isLoading, login } = useUser();
    const [isAutoLoggingIn, setIsAutoLoggingIn] = React.useState(
        () => AUTOLOGIN_ENABLED && !autoLoginAttempted,
    );

    React.useEffect(() => {
        if (isLoading || !AUTOLOGIN_ENABLED || autoLoginAttempted || isAuthenticated()) {
            // Nothing to do: either already authenticated, autologin disabled, or already tried.
            if (isAutoLoggingIn) {
                setIsAutoLoggingIn(false);
            }

            return;
        }

        autoLoginAttempted = true;
        setIsAutoLoggingIn(true);

        let cancelled = false;

        (async () => {
            try {
                await login({ mail: AUTOLOGIN_EMAIL as string, password: AUTOLOGIN_PASSWORD as string });
            } catch {
                // Silent failure: fall back to the normal login page.
            } finally {
                if (!cancelled) {
                    setIsAutoLoggingIn(false);
                }
            }
        })();

        return () => {
            cancelled = true;
        };
    }, [isLoading, isAuthenticated, login, isAutoLoggingIn]);

    if (isLoading || isAutoLoggingIn) {
        return (
            <div className="flex h-dvh w-full items-center justify-center">
                <Loader2 className="text-muted-foreground size-8 animate-spin" />
            </div>
        );
    }

    if (!isAuthenticated()) {
        const returnParam = getReturnUrlParam(location.pathname);

        return (
            <Navigate
                replace
                to={`/login${returnParam}`}
            />
        );
    }

    return children;
}

export default ProtectedRoute;
