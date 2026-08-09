import { act, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { api } from '@/lib/axios';

import { AUTH_STORAGE_KEY, UserProvider, useUser } from './user-provider';

vi.mock('@/lib/axios', () => ({
    api: {
        get: vi.fn(),
    },
    getApiErrorStatusCode: vi.fn(() => 502),
}));

vi.mock('sonner', () => ({
    toast: {
        error: vi.fn(),
        info: vi.fn(),
        success: vi.fn(),
        warning: vi.fn(),
    },
}));

const cachedAuth = {
    expires_at: '2099-01-01T00:00:00.000Z',
    type: 'user' as const,
    user: {
        created_at: '2026-01-01T00:00:00.000Z',
        hash: 'cached',
        id: 1,
        mail: 'admin@pentagi.com',
        name: 'admin',
        password_change_required: false,
        provide: 'local',
        role_id: 1,
        status: 'active' as const,
        type: 'local' as const,
    },
};

const storage = new Map<string, string>();

Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: {
        clear: () => storage.clear(),
        getItem: (key: string) => storage.get(key) ?? null,
        removeItem: (key: string) => storage.delete(key),
        setItem: (key: string, value: string) => storage.set(key, value),
    },
});

function Probe() {
    const { backendUnreachable } = useUser();

    return <div>{backendUnreachable ? 'starting' : 'ready'}</div>;
}

describe('UserProvider cold-start recovery', () => {
    beforeEach(() => {
        vi.useFakeTimers();
        localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(cachedAuth));
        vi.mocked(api.get)
            .mockRejectedValueOnce(new Error('gateway starting'))
            .mockRejectedValueOnce(new Error('gateway starting'))
            .mockResolvedValue({ data: cachedAuth, status: 'success' });
    });

    afterEach(() => {
        vi.useRealTimers();
        vi.clearAllMocks();
        localStorage.clear();
    });

    it('keeps polling info for a cached user until the backend recovers', async () => {
        render(
            <MemoryRouter initialEntries={['/flows/1']}>
                <UserProvider>
                    <Probe />
                </UserProvider>
            </MemoryRouter>,
        );

        await act(async () => {
            await Promise.resolve();
        });
        expect(screen.getByText('starting')).toBeInTheDocument();

        await act(async () => {
            await vi.advanceTimersByTimeAsync(6000);
        });

        expect(api.get).toHaveBeenCalledTimes(3);
        expect(screen.getByText('ready')).toBeInTheDocument();
    });
});
