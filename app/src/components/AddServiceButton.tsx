import { ArkSheet } from "../ark/ArkSheet";
import { createSignal, untrack } from "solid-js";
import ServicePlusSVG from "/ServicePlus.svg";
import { fetchServices } from "../shared";
import { toaster } from "../ark/ArkToast";
import { ServiceForm } from "./ServiceForm";
import { serviceClient } from "../lib/rpc";

interface AddServiceButtonProps {
    nodeId: string;
}

export default function AddServiceButton(props: AddServiceButtonProps) {
    const [name, setName] = createSignal("");
    const [serviceUrl, setServiceUrl] = createSignal("");
    const [isSubmitting, setIsSubmitting] = createSignal(false);

    const handleSave = async (closeDialog: () => void) => {
        const _name = untrack(name).trim();
        const _serviceUrl = untrack(serviceUrl).trim();

        if (!_name || !_serviceUrl) {
            toaster.create({
                title: 'Validation Error',
                description: 'Please fill in all required fields.',
                type: 'error',
            });
            return;
        }

        setIsSubmitting(true);

        try {
            await serviceClient.createService({
                name: _name,
                url: _serviceUrl,
                nodeId: props.nodeId,
            });

            await fetchServices(props.nodeId);

            toaster.create({
                title: 'Success!',
                description: 'Service has been created successfully.',
                type: 'success',
            });

            setName("");
            setServiceUrl("");
            closeDialog();
        } catch (error) {
            console.error('Failed to create service:', error);
            toaster.create({
                title: 'Failed',
                description: 'There was an error creating your service. Please try again.',
                type: 'error',
            });
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <ArkSheet
            trigger={(_, setOpen) => (
                <button
                    onClick={() => setOpen(true)}
                    class="w-full drop-shadow-2xl px-4 py-2 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 flex items-center space-x-2 justify-center"
                >
                    <img src={ServicePlusSVG} class="w-5 h-5" style="filter: brightness(0) invert(1)" />
                    <span class="line-clamp-1 break-all">Add Service</span>
                </button>
            )}
            title="Add a new service"
            description="Configure your video service."
        >
            {(setOpen) => (
                <ServiceForm
                    name={name}
                    setName={setName}
                    serviceUrl={serviceUrl}
                    setServiceUrl={setServiceUrl}
                    onSubmit={() => handleSave(() => setOpen(false))}
                    isSubmitting={isSubmitting()}
                />
            )}
        </ArkSheet>
    );
}
