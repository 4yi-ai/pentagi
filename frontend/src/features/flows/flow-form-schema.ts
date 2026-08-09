import { z } from 'zod';

export const flowFormSchema = z
    .object({
        message: z.string().trim(),
        providerName: z.string().trim().min(1, { message: 'Provider must be selected' }),
        resourceIds: z.array(z.string()),
        useAgents: z.boolean(),
    })
    .refine(({ message, resourceIds }) => message.length > 0 || resourceIds.length > 0, {
        message: 'Add a message or attach at least one file',
        path: ['message'],
    });

export type FlowFormValues = z.infer<typeof flowFormSchema>;

const ATTACHMENT_ONLY_INPUT = 'Please analyze the attached file or files.';

export const getFlowFormInput = ({ message, resourceIds }: Pick<FlowFormValues, 'message' | 'resourceIds'>) =>
    message.trim() || (resourceIds.length > 0 ? ATTACHMENT_ONLY_INPUT : '');
