import { zodResolver } from '@hookform/resolvers/zod';
import { FolderPlus } from 'lucide-react';
import { useEffect } from 'react';
import { useForm } from 'react-hook-form';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { FormSubmitButton } from '@/components/ui/form-submit-button';
import { Input } from '@/components/ui/input';
import { useT } from '@/lib/i18n';

import { resourcesMkdirFormSchema, type ResourcesMkdirFormValues, useResourcesMkdir } from './use-resources-mkdir';

interface ResourcesMkdirDialogFormProps {
    defaultParentPath: string;
    onClose: () => void;
}

interface ResourcesMkdirDialogProps {
    /** Pre-filled parent path (without leading "/"). Empty string targets the root. */
    defaultParentPath?: string;
    isOpen: boolean;
    onClose: () => void;
}

const buildDefaultPath = (defaultParentPath: string): string =>
    defaultParentPath ? `${defaultParentPath.replace(/\/+$/u, '')}/new-folder` : 'new-folder';

export function ResourcesMkdirDialog({ defaultParentPath = '', isOpen, onClose }: ResourcesMkdirDialogProps) {
    const handleDialogOpenChange = (nextOpen: boolean) => {
        if (!nextOpen) {
            onClose();
        }
    };

    return (
        <Dialog
            onOpenChange={handleDialogOpenChange}
            open={isOpen}
        >
            {isOpen && (
                <ResourcesMkdirDialogForm
                    defaultParentPath={defaultParentPath}
                    onClose={onClose}
                />
            )}
        </Dialog>
    );
}

function ResourcesMkdirDialogForm({ defaultParentPath, onClose }: ResourcesMkdirDialogFormProps) {
    const t = useT();
    const { isCreating, mkdir } = useResourcesMkdir();

    const form = useForm<ResourcesMkdirFormValues>({
        defaultValues: { path: buildDefaultPath(defaultParentPath) },
        mode: 'onChange',
        resolver: zodResolver(resourcesMkdirFormSchema),
    });

    useEffect(() => {
        form.reset({ path: buildDefaultPath(defaultParentPath) });
    }, [defaultParentPath, form]);

    const handleSubmit = form.handleSubmit(async (values) => {
        const wasCreated = await mkdir(values);

        if (wasCreated) {
            onClose();
        }
    });

    return (
        <DialogContent>
            <DialogHeader>
                <DialogTitle className="flex items-center gap-2">
                    <FolderPlus className="size-4" />
                    {t('res.mkdirTitle')}
                </DialogTitle>
                <DialogDescription>{t('res.mkdirDesc')}</DialogDescription>
            </DialogHeader>

            <Form {...form}>
                <form
                    className="flex flex-col gap-4"
                    onSubmit={handleSubmit}
                >
                    <FormField
                        control={form.control}
                        name="path"
                        render={({ field }) => (
                            <FormItem>
                                <FormLabel>{t('res.pathLabel')}</FormLabel>
                                <FormControl>
                                    <Input
                                        {...field}
                                        autoComplete="off"
                                        autoFocus
                                        disabled={isCreating}
                                        placeholder="reports/2025"
                                    />
                                </FormControl>
                                <FormDescription>
                                    {t('res.mkdirPathHelpPre')}
                                    <code>/</code>
                                    {t('res.mkdirPathHelpPost')}
                                </FormDescription>
                                <FormMessage />
                            </FormItem>
                        )}
                    />

                    <div className="flex justify-end gap-2">
                        <Button
                            disabled={isCreating}
                            onClick={onClose}
                            type="button"
                            variant="outline"
                        >
                            {t('res.cancel')}
                        </Button>
                        <FormSubmitButton icon={<FolderPlus />}>{t('res.create')}</FormSubmitButton>
                    </div>
                </form>
            </Form>
        </DialogContent>
    );
}
