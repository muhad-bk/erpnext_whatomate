<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'
import { cva, type VariantProps } from 'class-variance-authority'

const badgeVariants = cva(
  'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
  {
    variants: {
      variant: {
        // Modern opacity-based badges like Linear/Vercel
        default: 'border-transparent bg-primary/20 text-primary light:bg-primary/15 light:text-primary',
        secondary: 'border-transparent bg-muted text-muted-foreground',
        destructive: 'border-transparent bg-red-500/20 text-red-400 light:bg-red-100 light:text-red-700',
        outline: 'border-border text-foreground',
        success: 'border-transparent bg-primary/20 text-primary light:bg-primary/15 light:text-primary',
        warning: 'border-transparent bg-amber-500/20 text-amber-400 light:bg-amber-100 light:text-amber-700',
        info: 'border-transparent bg-primary/15 text-primary light:bg-primary/10 light:text-primary',
        active: 'border-transparent bg-primary/25 text-primary badge-pulse light:bg-primary/15 light:text-primary'
      }
    },
    defaultVariants: {
      variant: 'default'
    }
  }
)

type BadgeVariants = VariantProps<typeof badgeVariants>

const props = defineProps<{
  variant?: BadgeVariants['variant']
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <div :class="cn(badgeVariants({ variant }), props.class)">
    <slot />
  </div>
</template>
