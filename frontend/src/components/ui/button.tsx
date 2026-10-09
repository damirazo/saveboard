import type { ComponentProps } from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '../../lib/utils';

const buttonVariants = cva('ui-button', {
  variants: { variant: { default: 'primary-button', outline: 'outline-button', icon: 'icon-button', filter: 'filter-button', tag: 'tag-button' } },
  defaultVariants: { variant: 'default' },
});
export function Button({ className, variant, asChild = false, type = 'button', ...props }: ComponentProps<'button'> & VariantProps<typeof buttonVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : 'button';
  return <Comp {...(!asChild ? { type } : {})} className={cn(buttonVariants({ variant, className }))} {...props} />;
}
