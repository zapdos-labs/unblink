
import { AiOutlineVideoCameraAdd } from 'solid-icons/ai';
import { Dialog } from '@ark-ui/solid/dialog';
import { ArkDialog } from './ark/ArkDialog';
import { createSignal, untrack } from 'solid-js';
import { fetchCameras } from './shared';
import { toaster } from './ark/ArkToast';

export default function AddCameraButton() {
    const [name, setName] = createSignal('');
    const [uri, setUri] = createSignal('');
    const [labels, setLabels] = createSignal('');

    const handleSave = async () => {
        const _name = untrack(name).trim();
        const _uri = untrack(uri).trim();
        if (!_name || !_uri) {
            return;
        }

        const labelsArray = labels().split(',').map(l => l.trim()).filter(l => l);

        toaster.promise(async () => {
            const response = await fetch('/media', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ name: _name, uri: _uri, labels: labelsArray }),
            });

            if (response.ok) {
                setName('');
                setUri('');
                setLabels('');
                fetchCameras();
            } else {
                throw new Error('Failed to save camera');
            }
        }, {
            loading: {
                title: 'Saving...',
                description: 'Your camera is being added.',
            },
            success: {
                title: 'Success!',
                description: 'Camera has been added successfully.',
            },
            error: {
                title: 'Failed',
                description: 'There was an error adding your camera. Please try again.',
            },
        })
    };

    return <ArkDialog
        trigger={(_, setOpen) => <button
            onClick={() => setOpen(true)}
            class="w-full btn-primary">
            <AiOutlineVideoCameraAdd class="w-6 h-6" />
            <div>
                Add Camera
            </div>
        </button>}
        title="Add a new camera"
        description="Enter the details for your new camera."
    >
        <div class="mt-4 space-y-4">
            <div>
                <label for="camera-name" class="text-sm font-medium text-neu-300">Camera Name</label>
                <input
                    value={name()}
                    onInput={(e) => setName(e.currentTarget.value)}
                    placeholder='Front Gate'
                    type="text" id="camera-name" class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500" />
            </div>
            <div>
                <label for="camera-url" class="text-sm font-medium text-neu-300">Camera URL</label>
                <input
                    value={uri()}
                    onInput={(e) => setUri(e.currentTarget.value)}
                    placeholder='rtsp://localhost:8554/cam'
                    type="text" id="camera-url" class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500" />
            </div>
            <div>
                <label for="camera-labels" class="text-sm font-medium text-neu-300">Labels (comma-separated)</label>
                <input
                    value={labels()}
                    onInput={(e) => setLabels(e.currentTarget.value)}
                    placeholder='Outside, Security, Front Door'
                    type="text" id="camera-labels" class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500" />
            </div>
            <div class="flex justify-end pt-4">
                {/* There should be no asChild here */}
                <Dialog.CloseTrigger>
                    <button
                        onClick={handleSave}
                        class="btn-primary">
                        Save Camera
                    </button>
                </Dialog.CloseTrigger>
            </div>
        </div>
    </ArkDialog>
}