import { createSignal, untrack } from 'solid-js';
import { ArkSheet } from '../ark/ArkSheet';
import type { Service } from '../shared';
import { toaster } from '../ark/ArkToast';
import { serviceClient } from '../lib/rpc';

interface ServiceEditSheetProps {
  service: Service;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onServiceUpdated?: () => void;
}

export const ServiceEditSheet = (props: ServiceEditSheetProps) => {
  const [name, setName] = createSignal(props.service.name);
  const [serviceUrl, setServiceUrl] = createSignal(props.service.serviceUrl);
  const [isSubmitting, setIsSubmitting] = createSignal(false);

  const handleSave = async () => {
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
      await serviceClient.updateService({
        id: props.service.id,
        name: _name,
        url: _serviceUrl,
      });

      toaster.create({
        title: 'Success!',
        description: 'Service has been updated successfully.',
        type: 'success',
      });

      props.onOpenChange(false);
      props.onServiceUpdated?.();
    } catch (error) {
      console.error('Failed to update service:', error);
      toaster.create({
        title: 'Failed',
        description: 'There was an error updating your service. Please try again.',
        type: 'error',
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <ArkSheet
      trigger={() => <></>}
      title="Edit Service"
      description="View and edit service information"
      open={props.open}
      onOpenChange={props.onOpenChange}
    >
      {() => (
        <div class="space-y-6">
          {/* Service ID - Read-only */}
          <div>
            <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">
              Service ID
            </label>
            <p class="mt-1 text-sm text-white font-mono line-clamp-1 break-all">
              {props.service.id}
            </p>
          </div>

          {/* Service Name - Editable */}
          <div>
            <label for="edit-service-name" class="text-xs font-medium text-neu-500 uppercase tracking-wide">
              Service Name
            </label>
            <input
              value={name()}
              onInput={(e) => setName(e.currentTarget.value)}
              type="text"
              id="edit-service-name"
              class="mt-1 px-3 py-2 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          {/* Service URL - Editable */}
          <div>
            <label for="edit-service-url" class="text-xs font-medium text-neu-500 uppercase tracking-wide">
              Service URL
            </label>
            <input
              value={serviceUrl()}
              onInput={(e) => setServiceUrl(e.currentTarget.value)}
              type="text"
              id="edit-service-url"
              class="mt-1 px-3 py-2 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white text-sm font-mono focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          </div>

          {/* Node ID - Read-only */}
          <div>
            <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">
              Node ID
            </label>
            <p class="mt-1 text-sm text-white font-mono line-clamp-1 break-all">
              {props.service.nodeId}
            </p>
          </div>

          {/* Save Button */}
          <div class="flex-shrink-0 pt-2">
            <button
              onClick={() => handleSave()}
              disabled={!name() || !serviceUrl() || isSubmitting()}
              class="w-full py-3 rounded-lg font-semibold transition-all outline-none"
              classList={{
                "bg-white text-black hover:bg-neu-200": !!name() && !!serviceUrl() && !isSubmitting(),
                "bg-neu-700 text-neu-500 cursor-not-allowed": !name() || !serviceUrl() || isSubmitting(),
              }}
            >
              {isSubmitting() ? "Saving..." : "Save Changes"}
            </button>
          </div>
        </div>
      )}
    </ArkSheet>
  );
};

export default ServiceEditSheet;
