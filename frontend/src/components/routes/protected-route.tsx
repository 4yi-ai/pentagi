import { Loader2 } from 'lucide-react';
import * as React from 'react';
import { Navigate, useLocation } from 'react-router-dom';

import { getReturnUrlParam } from '@/lib/utils/auth';
import { useUser } from '@/providers/user-provider';

// Seamless no-login is handled entirely by the backend (AUTH_AUTO_LOGIN): when a
// deployment sits behind an external SSO gateway, the server transparently
// authenticates every request as the default admin, so the user is already
// authenticated by the time this guard runs and never sees the login page.
// When AUTH_AUTO_LOGIN is off, this guard falls back to the normal /login flow.
function ProtectedRoute({ children }: { children: React.ReactNode }) {
    const location = useLocation();
    const { isAuthenticated, isLoading } = useUser();

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
