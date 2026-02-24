
import { Toast, Toaster, createToaster } from '@ark-ui/solid/toast'
import { BsXLg } from 'solid-icons/bs'
import { FaSolidCircleCheck, FaSolidCircleExclamation, FaSolidSpinner } from 'solid-icons/fa'


export default function ArkToast() {
    const getIcon = (type: string | undefined) => {
        switch (type) {
            case 'loading':
                return <FaSolidSpinner size={20} class="animate-spin" />
            case 'success':
                return <FaSolidCircleCheck size={20} />
            case 'error':
                return <FaSolidCircleExclamation size={20} />
            default:
                return null
        }
    }

    return <Toaster toaster={toaster}>
        {(toast) => (
            <Toast.Root >
                <div class="flex items-start space-x-3 p-4 bg-neu-800 rounded-lg shadow min-w-sm max-w-sm">
                    {getIcon(toast().type)}
                    <div class="flex-1">
                        <Toast.Title>{toast().title}</Toast.Title>
                        <Toast.Description>{toast().description}</Toast.Description>
                    </div>
                    <Toast.CloseTrigger>
                        <BsXLg />
                    </Toast.CloseTrigger>
                </div>
            </Toast.Root>
        )}
    </Toaster>
}


export const toaster = createToaster({
    overlap: true,
    placement: 'bottom-end',
    gap: 16,
})
