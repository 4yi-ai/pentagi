import { beforeEach, expect, it, vi } from 'vitest';
import type { FlowFragmentFragment } from '@/graphql/types';
import axios from '@/lib/axios';
import { generateReportWithEvidence } from './report';
vi.mock('@/lib/axios', () => ({ default: { get: vi.fn(), post: vi.fn() } }));
const flow = { id: '2', title: '/work/evidence/test.md', status: 'finished' } as FlowFragmentFragment;
beforeEach(() => vi.resetAllMocks());
it('includes retrieved evidence and masks authentication headers', async () => {
    vi.mocked(axios.get).mockResolvedValue('Step 1: GET /test\nAuthorization: Bearer private\nHTTP 403');
    const result = await generateReportWithEvidence([], flow);
    expect(result).toContain('Step 1: GET /test');
    expect(result).toContain('HTTP 403');
    expect(result).not.toContain('Bearer private');
    expect(axios.post).not.toHaveBeenCalled();
});
it('pulls missing cached evidence and retries download', async () => {
    vi.mocked(axios.get).mockRejectedValueOnce(new Error('missing')).mockResolvedValueOnce('actual evidence');
    const result = await generateReportWithEvidence([], flow);
    expect(axios.post).toHaveBeenCalledWith('/flows/2/files/pull', { paths: ['/work/evidence/test.md'] });
    expect(result).toContain('actual evidence');
});
it('explicitly marks unavailable evidence', async () => {
    vi.mocked(axios.get).mockRejectedValue(new Error('missing'));
    vi.mocked(axios.post).mockRejectedValue(new Error('stopped'));
    expect(await generateReportWithEvidence([], flow)).toContain('证据文件未能读取');
});
