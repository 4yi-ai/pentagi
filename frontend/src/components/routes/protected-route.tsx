import { Loader2 } from 'lucide-react';
import * as React from 'react';
import { Navigate, useLocation } from 'react-router-dom';

import { useT } from '@/lib/i18n';
import { getReturnUrlParam } from '@/lib/utils/auth';
import { useUser } from '@/providers/user-provider';

// Seamless no-login is handled entirely by the backend (AUTH_AUTO_LOGIN): when a
// deployment sits behind an external SSO gateway, the server transparently
// authenticates every request as the default admin, so the user is already
// authenticated by the time this guard runs and never sees the login page.
// When AUTH_AUTO_LOGIN is off, this guard falls back to the normal /login flow.
function ProtectedRoute({ children }: { children: React.ReactNode }) {
    const location = useLocation();
    const t = useT();
    const { backendUnreachable, isAuthenticated, isLoading } = useUser();

    // Backend unreachable (pod resuming/starting after idle, gateway 5xx, or a
    // network blip): show a "starting up" screen while /info keeps polling, rather
    // than the login page — this deployment has no credentials to log in with.
    if (backendUnreachable) {
        return (
            <div className="flex h-dvh w-full flex-col items-center justify-center gap-4 px-6 text-center">
                <Loader2 className="text-muted-foreground size-8 animate-spin" />
                <div className="space-y-1">
                    <p className="text-foreground text-sm font-medium">{t('app.startingUp')}</p>
                    <p className="text-muted-foreground max-w-sm text-xs">{t('app.startingUpHint')}</p>
                </div>
            </div>
        );
    }

    if (isLoading) {
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
