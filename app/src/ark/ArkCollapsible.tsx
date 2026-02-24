import { Collapsible } from '@ark-ui/solid/collapsible'
import { FaSolidChevronRight } from 'solid-icons/fa'
import { createSignal } from 'solid-js'

export const ArkCollapsible = (props: {
    children: any
    toggle: any
    class?: string
    defaultOpen?: boolean
}) => {
    const [open, setOpen] = createSignal(props.defaultOpen ?? false)

    return (
        <Collapsible.Root open={open()} onOpenChange={(details) => setOpen(details.open)}>
            <Collapsible.Trigger
                class={'w-full flex items-center gap-2 focus:outline-none ' + (props.class ? props.class : '')}>
                <Collapsible.Indicator>
                    <div
                        data-open={open()}
                        class="transition-transform data-[open=true]:rotate-90"
                    >
                        <FaSolidChevronRight />
                    </div>
                </Collapsible.Indicator>
                {props.toggle}
            </Collapsible.Trigger>
            <Collapsible.Content>{props.children}</Collapsible.Content>
        </Collapsible.Root>
    )
}
