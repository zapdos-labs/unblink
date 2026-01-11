import { Tooltip } from '@ark-ui/solid/tooltip'
import type { JSX } from 'solid-js'

export const ArkTooltip = (props: {
    trigger: JSX.Element
    content: JSX.Element
    positioning?: {
        placement?: 'top' | 'top-start' | 'top-end' | 'bottom' | 'bottom-start' | 'bottom-end' | 'left' | 'left-start' | 'left-end' | 'right' | 'right-start' | 'right-end'
        offset?: { mainAxis?: number; crossAxis?: number }
    }
    open?: boolean
    onOpenChange?: (details: { open: boolean }) => void
}) => {
    return (
        <Tooltip.Root
            positioning={props.positioning}
            open={props.open}
            onOpenChange={props.onOpenChange}
        >
            <Tooltip.Trigger>{props.trigger}</Tooltip.Trigger>
            <Tooltip.Positioner>
                <Tooltip.Content class="z-50 px-2 py-1 text-xs font-medium text-white bg-neu-950/90 border border-neu-700 rounded shadow-lg backdrop-blur-sm">
                    {props.content}
                </Tooltip.Content>
            </Tooltip.Positioner>
        </Tooltip.Root>
    )
}
