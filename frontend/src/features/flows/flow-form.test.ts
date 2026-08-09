import { describe, expect, it } from 'vitest';

import { flowFormSchema, getFlowFormInput } from './flow-form-schema';

const baseValues = {
    providerName: 'custom',
    resourceIds: [] as string[],
    useAgents: false,
};

describe('flowFormSchema', () => {
    it('accepts an attachment without a text message', () => {
        const result = flowFormSchema.safeParse({ ...baseValues, message: '', resourceIds: ['42'] });

        expect(result.success).toBe(true);
    });

    it('rejects a submission without a message or attachment', () => {
        const result = flowFormSchema.safeParse({ ...baseValues, message: '   ' });

        expect(result.success).toBe(false);
    });
});

describe('getFlowFormInput', () => {
    it('uses the typed message when present', () => {
        expect(getFlowFormInput({ message: '  inspect this image  ', resourceIds: ['42'] })).toBe(
            'inspect this image',
        );
    });

    it('provides an instruction for attachment-only submissions', () => {
        expect(getFlowFormInput({ message: '', resourceIds: ['42'] })).toBe(
            'Please analyze the attached file or files.',
        );
    });
});
