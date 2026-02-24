
import { Dialog } from '@ark-ui/solid/dialog'
import { BsX } from 'solid-icons/bs'
import { createSignal, type JSX, createEffect } from 'solid-js'
import { Portal } from 'solid-js/web'
import { useMaxWidth } from '../hooks/useMaxWidth'

export const ArkSheet = (props: {
  trigger: (open: boolean, setOpen: (open: boolean) => void) => JSX.Element
  title: string
  description?: string
  children: JSX.Element | ((setOpen: (open: boolean) => void) => JSX.Element)
  open?: boolean
  onOpenChange?: (open: boolean) => void
}) => {
  const getMaxWidth = useMaxWidth()
  const [open, setOpen] = createSignal(props.open ?? false)

  // Sync with external open prop
  createEffect(() => {
    if (props.open !== undefined && props.open !== open()) {
      setOpen(props.open)
    }
  })

  const handleOpenChange = (newOpen: boolean) => {
    setOpen(newOpen)
    props.onOpenChange?.(newOpen)
  }
  const [dragY, setDragY] = createSignal(0)
  const [isDragging, setIsDragging] = createSignal(false)
  const [startY, setStartY] = createSignal(0)
  let dragContainerRef: HTMLDivElement | undefined

  const handlePointerDown = (e: PointerEvent) => {
    setIsDragging(true)
    setStartY(e.clientY)
    ; (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }

  const handlePointerMove = (e: PointerEvent) => {
    if (!isDragging()) return
    const delta = e.clientY - startY()
    if (delta > 0) setDragY(delta) // Only allow drag down
  }

  const handlePointerUp = (e: PointerEvent) => {
    setIsDragging(false)
      ; (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId)

    const shouldDismiss = dragY() > 100

    if (shouldDismiss) {
      handleOpenChange(false) // Dismiss
      setDragY(0) // Reset
    } else {
      // Spring back with animation (controlled by style object)
      setDragY(0)
    }
  }

  return (
    <>
      {props.trigger(open(), handleOpenChange)}
      <Dialog.Root open={open()} onOpenChange={(details) => handleOpenChange(details.open)}>
        <Portal>
          <Dialog.Backdrop
            class="fixed inset-0 bg-black/80 z-50 transition-opacity duration-300 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0"
            onClick={(e) => e.stopPropagation()}
          />
          <Dialog.Positioner
            class="fixed inset-0 flex items-end justify-center z-50"
            onClick={(e) => e.stopPropagation()}
          >
            <Dialog.Content
              class={`relative w-full h-[70vh] flex flex-col ${getMaxWidth()}`}
              onClick={(e) => e.stopPropagation()}
            >
              <div
                ref={dragContainerRef}
                class="flex flex-col flex-1 min-h-0 bg-neu-900 rounded-t-3xl shadow-lg"
                style={{
                  transform: isDragging() || dragY() > 0 ? `translateY(${dragY()}px)` : undefined,
                  transition: isDragging() ? 'none' : 'transform 0.3s cubic-bezier(0.4, 0, 0.2, 1)'
                }}
              >
                {/* Draggable header region */}
                <div
                  class="flex-shrink-0 cursor-grab active:cursor-grabbing touch-none select-none"
                  onPointerDown={handlePointerDown}
                  onPointerMove={handlePointerMove}
                  onPointerUp={handlePointerUp}
                >
                  {/* Drag handle indicator */}
                  <div class="flex justify-center pt-3 pb-1">
                    <div class="w-12 h-1.5 bg-neu-700 rounded-full" />
                  </div>

                  {/* Title and description */}
                  <div class="px-6 pt-2 pb-4">
                    <Dialog.Title class="m-0 text-lg font-medium text-white">
                      {props.title}
                    </Dialog.Title>
                    {props.description && (
                      <Dialog.Description class="mt-2 text-sm leading-relaxed text-neu-400">
                        {props.description}
                      </Dialog.Description>
                    )}
                  </div>
                </div>

                {/* Scrollable content area */}
                <div class="flex-1 overflow-y-auto px-6 pb-6 min-h-0">
                  {typeof props.children === 'function' ? props.children(setOpen) : props.children}
                </div>

                {/* Close button - absolutely positioned */}
                <Dialog.CloseTrigger
                  class="absolute top-3 right-3 text-neu-400 hover:text-white focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 focus-visible:ring-offset-neu-800 p-1"
                  onClick={(e) => e.stopPropagation()}
                >
                  <BsX class="w-6 h-6" />
                </Dialog.CloseTrigger>
              </div>
            </Dialog.Content>
          </Dialog.Positioner>
        </Portal>
      </Dialog.Root>
    </>
  )
}
