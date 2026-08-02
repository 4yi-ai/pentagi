import brandLogo from '@/assets/brand-logo.png';
import { cn } from '@/lib/utils';

interface LogoProps extends React.ImgHTMLAttributes<HTMLImageElement> {
    className?: string;
}

function Logo({ className, ...props }: LogoProps) {
    return (
        <img
            alt="4YI Pentest"
            className={cn('object-contain', className)}
            src={brandLogo}
            {...props}
        />
    );
}

export default Logo;
