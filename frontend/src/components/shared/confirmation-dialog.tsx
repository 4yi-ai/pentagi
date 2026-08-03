import type { ReactElement } from 'react';

import { Loader2, Trash2 } from 'lucide-react';
import { cloneElement, isValidElement, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { useT } from '@/lib/i18n';
import { cn } from '@/lib/utils';

type ConfirmationDialogIconProps = ReactElement<React.SVGProps<SVGSVGElement>>;

interface ConfirmationDialogProps {
    cancelIcon?: ConfirmationDialogIconProps;
    cancelText?: string;
    cancelVariant?: 'default' | 'destructive' | 'ghost' | 'outline' | 'secondary';
    confirmIcon?: ConfirmationDialogIconProps;
    confirmText?: string;
    confirmVariant?: 'default' | 'destructive' | 'ghost' | 'outline' | 'secondary';
    description?: string;
    /** May be sync or async. If async, the dialog keeps itself open and shows a spinner until the promise settles. */
    handleConfirm: () => Promise<void> | void;
    handleOpenChange: (isOpen: boolean) => void;
    isOpen: boolean;
    itemName?: string;
    itemType?: string;
    title?: string;
}

function ConfirmationDialog({
    cancelIcon,
    cancelText,
    cancelVariant = 'outline',
    confirmIcon = <Trash2 />,
    confirmText,
    confirmVariant = 'destructive',
    description,
    handleConfirm,
    handleOpenChange,
    isOpen,
    itemName,
    itemType,
    title,
}: ConfirmationDialogProps) {
    const t = useT();
    const [isProcessing, setIsProcessing] = useState(false);

    const cancelLabel = cancelText ?? t('cancel');
    const confirmLabel = confirmText ?? t('confirmConfirm');
    const resolvedItemName = itemName ?? t('confirmItemNameDefault');
    const resolvedItemType = itemType ?? t('confirmItemTypeDefault');

    // Derive a contextual title from confirm verb + item type so callers don't
    // see "Confirm Action" for a Delete prompt or a Save prompt. Explicit
    // `title` always wins.
    const verb = confirmText?.trim() ?? '';
    const resolvedTitle =
        title ??
        (verb
            ? t('confirmTitleAction').replace('{verb}', verb).replace('{itemType}', resolvedItemType)
            : t('confirmActionDefault'));

    const defaultDescription = description || (
        <>
            {t('confirmDescPrefix').replace('{verb}', verb.toLowerCase() || t('confirmDescFallbackVerb'))}{' '}
            <strong className="text-foreground font-semibold">{resolvedItemName}</strong> {resolvedItemType}
            {t('confirmDescSuffix')}
        </>
    );

    const processIcon = (icon?: ConfirmationDialogIconProps): ConfirmationDialogIconProps | null => {
        if (!icon) {
            return null;
        }

        if (isValidElement(icon)) {
            const { className = '', ...restProps } = icon.props;

            return cloneElement(icon, {
                ...restProps,
                className: cn('size-4', className),
            });
        }

        return icon;
    };

    const handleConfirmClick = async () => {
        if (isProcessing) {
            return;
        }

        setIsProcessing(true);

        try {
            await handleConfirm();
            handleOpenChange(false);
        } finally {
            setIsProcessing(false);
        }
    };

    return (
        <Dialog
            onOpenChange={(nextOpen) => {
                if (isProcessing) {
                    return;
                }

                handleOpenChange(nextOpen);
            }}
            open={isOpen}
        >
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>{resolvedTitle}</DialogTitle>
                    <DialogDescription>{defaultDescription}</DialogDescription>
                </DialogHeader>

                <DialogFooter>
                    <Button
                        disabled={isProcessing}
                        onClick={() => handleOpenChange(false)}
                        variant={cancelVariant}
                    >
                        {processIcon(cancelIcon)}
                        {cancelLabel}
                    </Button>
                    <Button
                        disabled={isProcessing}
                        onClick={() => {
                            void handleConfirmClick();
                        }}
                        variant={confirmVariant}
                    >
                        {isProcessing ? <Loader2 className="size-4 animate-spin" /> : processIcon(confirmIcon)}
                        {confirmLabel}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

export default ConfirmationDialog;
