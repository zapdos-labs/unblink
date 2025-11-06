import { BsMailbox2, BsPlugin, BsWhatsapp } from "solid-icons/bs";
import { createSignal, type Accessor, type Setter } from "solid-js";
import { ArkCollapsible } from "~/src/ark/ArkCollapsible";
import type { UseSubTab } from "../SettingsContent";


function ToggleLayout(props: {
    children: any
    title: string
    icon: any
    coming_soon?: boolean
}) {

    return <ArkCollapsible

        class="px-6 py-3 text-neu-400 hover:text-white transition-all "
        toggle={<div class="p-2 flex-1">
            <div class="float-left flex items-center space-x-3">
                <props.icon class="w-5 h-5" />
                <div class="line-clamp-1">{props.title}</div>
            </div>
        </div>}
    >
        <div
            data-comingsoon={props.coming_soon ? true : false}
            class="px-6 pb-4 space-y-4 data-[comingsoon=true]:opacity-50 data-[comingsoon=true]:pointer-events-none">
            {
                props.children
            }
        </div>

    </ArkCollapsible>

}

function WebhookToggle(props: {
    scratchpad: Accessor<Record<string, string>>,
    setScratchpad: Setter<Record<string, string>>,
}) {


    return <ToggleLayout
        title="Webhook Endpoint"
        icon={BsPlugin}
    >

        <div >
            <label for="camera-url" class="text-sm font-medium text-neu-300">Callback URL</label>
            <input
                value={
                    props.scratchpad()['alerts.webhook_callback_url'] || ''
                }
                onInput={(e) => {
                    props.setScratchpad(s => ({
                        ...s,
                        'alerts.webhook_callback_url': e.currentTarget.value,
                    }));
                }}
                placeholder='https://yourdomain.com/webhook'
                type="text" id="camera-url" class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500" />
        </div>
    </ToggleLayout>
}

function WhatsAppToggle() {
    const [phoneNumber, setPhoneNumber] = createSignal('');
    return <ToggleLayout
        coming_soon
        title="WhatsApp (coming soon)"
        icon={BsWhatsapp}
    >

        <div>
            <label for="phone-number" class="text-sm font-medium text-neu-300">Phone Number</label>
            <input
                value={phoneNumber()}
                onInput={(e) => setPhoneNumber(e.currentTarget.value)}
                placeholder='+1234567890'
                type="text" id="phone-number" class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500" />
        </div>
    </ToggleLayout>

}

function EmailToggle() {
    const [email, setEmail] = createSignal('');
    return <ToggleLayout
        coming_soon
        title="Email (coming soon)"
        icon={BsMailbox2}
    >

        <div>
            <label for="email" class="text-sm font-medium text-neu-300">Email Address</label>
            <input
                value={email()}
                onInput={(e) => setEmail(e.currentTarget.value)}
                placeholder='you@example.com'
                type="text" id="email" class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500" />
        </div>
    </ToggleLayout>

}


export function useAlertsSubTab(props: {
    scratchpad: Accessor<Record<string, string>>,
    setScratchpad: Setter<Record<string, string>>,
}): UseSubTab {
    return {
        comp: () => <div class="border border-neu-800 rounded-lg bg-neu-850 ">
            <div class="space-y-2 divide-y divide-neu-800">
                <WebhookToggle scratchpad={props.scratchpad} setScratchpad={props.setScratchpad} />
                <WhatsAppToggle />
                <EmailToggle />
            </div>
        </div>,
        keys: () => [
            {
                name: 'alerts.webhook_callback_url',
                normalize: (val: string) => val.trim(),
                validate: (val: string) => {
                    if (val === '') {
                        return {
                            type: 'success',
                        };
                    }
                    try {
                        const url = new URL(val);
                        if (url.protocol !== 'http:' && url.protocol !== 'https:') {
                            return {
                                type: 'error',
                                message: 'URL must start with http:// or https://'
                            };
                        }
                        return {
                            type: 'success',
                        };
                    } catch {
                        return {
                            type: 'error',
                            message: 'Invalid URL'
                        }
                    }
                },
            },
        ]
    }
}