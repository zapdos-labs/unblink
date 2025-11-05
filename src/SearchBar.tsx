import {
    createEffect,
    createSignal,
    Show
} from "solid-js";
import Backdrop from "./search/Backdrop";
import SearchDropdown from "./search/SearchDropdown";
import SearchInput from "./search/SearchInput";
import { usePlaceholder } from "./search/usePlaceholder";
import { setState } from "./search/utils";
import { Portal } from "solid-js/web";



export default function SearchBar(props?: { variant?: "md" | "lg" | 'xl', placeholder?: () => string | undefined | null }) {
    const variant = () => props?.variant || "md";
    const { placeholder } = usePlaceholder({
        placeholder: props?.placeholder
    });
    const [isOpen, setIsOpen] = createSignal(false);
    const [barRef, setBarRef] = createSignal<HTMLDivElement>();
    const [query, setQuery] = createSignal("");

    let searchTimeout: any = null;
    createEffect(() => {
        const q = query().trim();
        if (searchTimeout) clearTimeout(searchTimeout);

        if (q === "") {
            setState({ type: "idle", result: { items: [] } });
            return;
        }

        setState({ type: "autocompleting", query: q });

        searchTimeout = setTimeout(async () => {
            try {
                const response = await fetch(`/api/autocomplete`, {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify({ text: q }),
                });
                if (!response.ok) {
                    setState({ type: "idle", query: q, autocomplete: { items: [] } }); // Show empty result on error
                    throw new Error("Search request failed");
                }
                const data = await response.json();
                console.log("Suggestion results:", data);

                setState({
                    type: "idle",
                    query: q,
                    autocomplete: { items: data.items || [] },
                });
            } catch (error) {
                console.error("Failed to fetch search results:", error);
                setState({ type: "idle", query: q, autocomplete: { items: [] } }); // Show empty result on error
            }
        }, 500);
    });

    createEffect(() => {
        const open = isOpen();
        if (!open) {
            setState({ type: "idle" });
            setQuery("");
        }
    });

    function doSubmit(query: string) {
        setIsOpen(false);
        console.log('query', query)
        // setTabId({ type: "search-result", query });
    }

    return (

        <div>
            <Backdrop isOpen={isOpen} barRef={barRef} setIsOpen={setIsOpen} />

            <div
                ref={setBarRef}
                data-variant={variant()}
                data-open={isOpen()}
                class="z-200 absolute top-1 left-1/2 -translate-x-1/2 w-[24rem] data-[variant=lg]:w-[40vw] data-[variant=xl]:w-[48vw] data-[open=true]:top-10 transition-[top,width,box-shadow] duration-300 ease-in-out data-[open=true]:w-[50vw] data-[variant=lg]:data-[open=true]:w-[50vw] data-[variant=xl]:data-[open=true]:w-[60vw] data-[open=true]:drop-shadow-lg  border border-neu-750  data-[open=false]:rounded-full data-[open=true]:rounded-2xl overflow-hidden group-data-[scheme=lighter]:bg-neu-800 bg-neu-800 data-[open=true]:bg-neu-900 "
            >
                <SearchInput
                    onSubmit={doSubmit}
                    query={query}
                    setQuery={setQuery}
                    isOpen={isOpen}
                    variant={variant}
                    placeholder={placeholder}
                />

                <Show when={isOpen()}>

                    <SearchDropdown
                        query={query} selectItem={(item) => {
                            doSubmit(item.text);
                        }} />

                </Show>
            </div>
        </div>
    );
}
