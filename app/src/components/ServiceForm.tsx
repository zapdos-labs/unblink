interface ServiceFormProps {
    name: () => string;
    setName: (name: string) => void;
    serviceUrl: () => string;
    setServiceUrl: (url: string) => void;
    onSubmit?: () => void;
    isSubmitting?: boolean;
}

// URL format examples for help text
const URL_EXAMPLES = [
    { type: "RTSP with auth", url: "rtsp://admin:password@192.168.1.100:554/stream" },
    { type: "RTSP without auth", url: "rtsp://192.168.1.100:554/stream" },
    { type: "HTTP with auth", url: "http://admin:pass@192.168.1.100:8080/video" },
    { type: "HTTP without auth", url: "http://192.168.1.100:8080/video" },
];

export function ServiceForm(props: ServiceFormProps) {
    return (
        <div class="space-y-6">
            <div>
                <h3 class="text-sm font-semibold text-neu-200 mb-3">Service Configuration</h3>
                <div class="space-y-4">
                    <div>
                        <label for="service-name" class="text-sm font-medium text-neu-300">
                            Service Name
                        </label>
                        <input
                            value={props.name()}
                            onInput={(e) => props.setName(e.currentTarget.value)}
                            placeholder="My Service"
                            type="text"
                            id="service-name"
                            class="px-4 py-2 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500"
                        />
                    </div>

                    {/* Service URL field */}
                    <div>
                        <label for="service-url" class="text-sm font-medium text-neu-300">
                            Service URL
                        </label>

                        <p class="text-xs text-neu-500 mt-1">
                            Enter the complete URL for your service (rtsp://, https://, etc.)
                        </p>
                        <input
                            value={props.serviceUrl()}
                            onInput={(e) => props.setServiceUrl(e.currentTarget.value)}
                            placeholder="rtsp://admin:password@192.168.1.100:554/stream"
                            type="text"
                            id="service-url"
                            class="px-4 py-2 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500 font-mono text-sm"
                        />
                    </div>

                    {/* URL Examples / Help */}
                    <div class="bg-neu-900 rounded-lg border border-neu-750 p-3 overflow-x-auto">
                        <p class="text-xs font-medium text-neu-300 mb-2">URL Format Examples:</p>
                        <div class="space-y-1 min-w-max">
                            {URL_EXAMPLES.map(example => (
                                <div class="flex items-start gap-2">
                                    <span class="text-xs text-neu-400 w-32 flex-shrink-0">{example.type}:</span>
                                    <code class="text-xs text-neu-500 font-mono whitespace-nowrap">{example.url}</code>
                                </div>
                            ))}
                        </div>
                    </div>
                </div>
            </div>

            {/* Submit Button */}
            <div class="flex-shrink-0">
                <button
                    onClick={() => props.onSubmit?.()}
                    disabled={!props.name() || !props.serviceUrl() || props.isSubmitting}
                    class="w-full py-3 rounded-lg font-semibold transition-all outline-none"
                    classList={{
                        "bg-white text-black hover:bg-neu-200": !!props.name() && !!props.serviceUrl() && !props.isSubmitting,
                        "bg-neu-700 text-neu-500 cursor-not-allowed": !props.name() || !props.serviceUrl() || props.isSubmitting,
                    }}
                >
                    {props.isSubmitting ? "Saving..." : "Add Service"}
                </button>
            </div>
        </div>
    );
}
