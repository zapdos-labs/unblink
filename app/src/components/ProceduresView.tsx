import type { SOPProcedure } from "@/gen/service/v1/service_pb";
import { formatDistanceToNow } from "date-fns";
import { FiMoreVertical, FiPlus, FiTrash2 } from "solid-icons/fi";
import { createSignal, For, onMount, Show } from "solid-js";
import { ArkMenu } from "../ark/ArkMenu";
import { serviceClient } from "../lib/rpc";
import { toaster } from "../ark/ArkToast";
import { ArkSheet } from "../ark/ArkSheet";

interface ProceduresViewProps {
  nodeId: string;
}

interface SOPTemplate {
  id: string;
  title: string;
  content: string;
}

const SOP_TEMPLATES: SOPTemplate[] = [
  {
    id: "container-belt-compliance",
    title: "Container Opening Belt Compliance",
    content:
      "If any worker attempts to open a container and no visible color safety belt is present, mark this as an SOP violation and should-alert event.",
  },
  {
    id: "forklift-proximity-safety",
    title: "Forklift Proximity Safety",
    content:
      "If any worker is inside a forklift laser boundary or appears within roughly 1-2 meters of a moving forklift, mark this as an unsafe proximity violation and should-alert event.",
  },
];

export default function ProceduresView(props: ProceduresViewProps) {
  const [loading, setLoading] = createSignal(true);
  const [saving, setSaving] = createSignal(false);
  const [procedures, setProcedures] = createSignal<SOPProcedure[]>([]);
  const [dialogOpen, setDialogOpen] = createSignal(false);
  const [templateDialogOpen, setTemplateDialogOpen] = createSignal(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = createSignal(false);
  const [deleting, setDeleting] = createSignal(false);
  const [pendingDeleteProcedure, setPendingDeleteProcedure] = createSignal<SOPProcedure | null>(null);
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [title, setTitle] = createSignal("");
  const [content, setContent] = createSignal("");

  const loadProcedures = async () => {
    setLoading(true);
    try {
      const res = await serviceClient.listSOPProceduresByNodeId({ nodeId: props.nodeId });
      setProcedures(res.procedures ?? []);
    } catch (error) {
      console.error("Failed to load SOP procedures:", error);
      toaster.create({
        title: "Failed to load procedures",
        description: "Could not fetch SOP procedures.",
        type: "error",
      });
    } finally {
      setLoading(false);
    }
  };

  const openCreateDialog = () => {
    setEditingId(null);
    setTitle("");
    setContent("");
    setTemplateDialogOpen(false);
    setDialogOpen(true);
  };

  const openEditDialog = (procedure: SOPProcedure) => {
    setEditingId(procedure.id);
    setTitle(procedure.title);
    setContent(procedure.content);
    setTemplateDialogOpen(false);
    setDialogOpen(true);
  };

  const applyTemplate = (template: SOPTemplate) => {
    setTitle(template.title);
    setContent(template.content);
    setTemplateDialogOpen(false);
  };

  const saveProcedure = async () => {
    const trimmedTitle = title().trim();
    const trimmedContent = content().trim();
    if (!trimmedTitle || !trimmedContent) {
      toaster.create({
        title: "Validation error",
        description: "Title and SOP text are required.",
        type: "error",
      });
      return;
    }

    setSaving(true);
    try {
      const id = editingId();
      if (id) {
        await serviceClient.updateSOPProcedure({
          id,
          title: trimmedTitle,
          content: trimmedContent,
        });
      } else {
        await serviceClient.createSOPProcedure({
          nodeId: props.nodeId,
          title: trimmedTitle,
          content: trimmedContent,
        });
      }
      await loadProcedures();
      setDialogOpen(false);
    } catch (error) {
      console.error("Failed to save SOP procedure:", error);
      toaster.create({
        title: "Failed to save",
        description: "Could not save SOP procedure.",
        type: "error",
      });
    } finally {
      setSaving(false);
    }
  };

  const openDeleteDialog = (procedure: SOPProcedure) => {
    setPendingDeleteProcedure(procedure);
    setDeleteDialogOpen(true);
  };

  const deleteProcedure = async () => {
    const procedure = pendingDeleteProcedure();
    if (!procedure) return;
    setDeleting(true);
    try {
      await serviceClient.deleteSOPProcedure({ id: procedure.id });
      await loadProcedures();
      setDeleteDialogOpen(false);
      setPendingDeleteProcedure(null);
    } catch (error) {
      console.error("Failed to delete SOP procedure:", error);
      toaster.create({
        title: "Failed to delete",
        description: "Could not delete SOP procedure.",
        type: "error",
      });
    } finally {
      setDeleting(false);
    }
  };

  const formatTimestamp = (procedure: SOPProcedure) => {
    const ts = procedure.updatedAt ?? procedure.createdAt;
    if (!ts) return "No timestamp";
    const seconds = Number(ts.seconds ?? 0n);
    const nanos = ts.nanos ?? 0;
    return formatDistanceToNow(new Date(seconds * 1000 + nanos / 1_000_000), {
      addSuffix: true,
    });
  };

  onMount(() => {
    void loadProcedures();
  });

  return (
    <div class="flex flex-col h-full bg-neu-900 text-white">
      <div class="flex items-center justify-between px-6 py-4 border-b border-neu-800">
        <div>
          <h1 class="text-xl font-semibold">Procedures</h1>
          <p class="text-sm text-neu-400 mt-1">Define SOP cards used by the VLM prompt.</p>
        </div>
        <button
          onClick={openCreateDialog}
          class="px-4 py-2 rounded-xl border border-neu-700 bg-neu-800 hover:bg-neu-750 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-neu-500 flex items-center gap-2"
        >
          <FiPlus class="w-4 h-4" />
          <span>Add SOP</span>
        </button>
      </div>

      <div class="flex-1 overflow-auto px-6 py-6">
        <Show
          when={!loading()}
          fallback={<div class="text-neu-400">Loading procedures...</div>}
        >
          <Show
            when={procedures().length > 0}
            fallback={
              <div class="rounded-2xl border border-neu-800 bg-neu-900/60 p-8 text-neu-400">
                No SOP yet. Add your first procedure.
              </div>
            }
          >
            <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              <For each={procedures()}>
                {(procedure) => (
                  <div
                    tabindex={0}
                    class="rounded-xl border border-neu-800 bg-neu-850 p-5 transition-colors hover:bg-neu-800 cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-neu-500"
                    onClick={(e) => {
                      if (!(e.target as HTMLElement).closest('[data-part="trigger"]')) {
                        openEditDialog(procedure);
                      }
                    }}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        openEditDialog(procedure);
                      }
                    }}
                  >
                    <div class="flex items-start justify-between gap-2">
                      <div class="text-lg font-medium line-clamp-1 pr-2">{procedure.title}</div>
                      <ArkMenu
                        items={() => [
                          { id: "delete", title: "Delete", icon: <FiTrash2 class="w-4 h-4" /> },
                        ]}
                        class="p-2 border border-neu-750 rounded-lg text-neu-400 hover:bg-neu-750 hover:border-neu-700 hover:text-white transition-colors"
                        triggerIcon={<FiMoreVertical class="w-4 h-4" />}
                        onSelect={(id) => {
                          if (id === "delete") {
                            openDeleteDialog(procedure);
                          }
                        }}
                        itemRender={(item) => (
                          <>
                            {item.icon}
                            <span>{item.title}</span>
                          </>
                        )}
                      />
                    </div>
                    <p class="text-sm text-neu-300 mt-2 line-clamp-4 whitespace-pre-wrap">
                      {procedure.content}
                    </p>
                    <div class="mt-4 text-xs text-neu-500">{formatTimestamp(procedure)}</div>
                  </div>
                )}
              </For>
            </div>
          </Show>
        </Show>
      </div>

      <ArkSheet
        trigger={() => <></>}
        title={editingId() ? "Edit SOP" : "New SOP"}
        description="This procedure text is included in the VLM instruction prompt."
        open={dialogOpen()}
        onOpenChange={(open) => {
          setDialogOpen(open);
          if (!open) {
            setTemplateDialogOpen(false);
          }
        }}
      >
        {() => (
          <div class="space-y-6">
            <button
              onClick={() => setTemplateDialogOpen(true)}
              class="px-3 py-1.5 rounded-xl border border-neu-700 bg-neu-800 hover:bg-neu-750 text-sm text-neu-100 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-neu-500"
            >
              Fill with template
            </button>
            <div>
              <label class="text-sm text-neu-300">Title</label>
              <input
                value={title()}
                onInput={(e) => setTitle(e.currentTarget.value)}
                placeholder="e.g. Forklift proximity safety"
                class="mt-1 w-full rounded-xl border border-neu-700 bg-neu-850 px-3 py-2 text-sm text-white focus:outline-none focus:border-neu-500 focus-visible:ring-2 focus-visible:ring-neu-500"
              />
            </div>
            <div>
              <label class="text-sm text-neu-300">SOP text</label>
              <textarea
                value={content()}
                onInput={(e) => setContent(e.currentTarget.value)}
                rows={8}
                placeholder="Describe the standard operating procedure..."
                class="mt-1 w-full rounded-xl border border-neu-700 bg-neu-850 px-3 py-2 text-sm text-white focus:outline-none focus:border-neu-500 focus-visible:ring-2 focus-visible:ring-neu-500 resize-y"
              />
            </div>
            <div class="flex-shrink-0 pt-2">
              <button
                onClick={() => void saveProcedure()}
                disabled={saving()}
                class="sheet-action-btn sheet-action-btn-primary"
              >
                {saving() ? "Saving..." : "Save"}
              </button>
            </div>
          </div>
        )}
      </ArkSheet>

      <ArkSheet
        trigger={() => <></>}
        title="Choose template"
        description="Select a starter SOP template and edit as needed."
        open={templateDialogOpen()}
        onOpenChange={setTemplateDialogOpen}
      >
        {() => (
          <div class="space-y-6">
            <div class="space-y-3">
              <For each={SOP_TEMPLATES}>
                {(template) => (
                  <button
                    onClick={() => applyTemplate(template)}
                    class="w-full text-left px-4 py-3 rounded-xl border border-neu-700 bg-neu-800 hover:bg-neu-750 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-neu-500"
                  >
                    <div class="font-medium text-white">{template.title}</div>
                    <p class="text-sm text-neu-300 mt-1 whitespace-pre-wrap">{template.content}</p>
                  </button>
                )}
              </For>
            </div>
          </div>
        )}
      </ArkSheet>

      <ArkSheet
        trigger={() => <></>}
        title="Delete SOP"
        description={pendingDeleteProcedure() ? `Delete "${pendingDeleteProcedure()!.title}"? This action cannot be undone.` : "This action cannot be undone."}
        open={deleteDialogOpen()}
        onOpenChange={(open) => {
          setDeleteDialogOpen(open);
          if (!open && !deleting()) {
            setPendingDeleteProcedure(null);
          }
        }}
      >
        {() => (
          <div class="space-y-6">
            <div class="text-sm text-neu-300">
              <Show when={pendingDeleteProcedure()} fallback={"Select an SOP to delete."}>
                {(procedure) => `You are about to permanently delete "${procedure().title}".`}
              </Show>
            </div>
            <div class="flex-shrink-0 pt-2">
              <button
                onClick={() => void deleteProcedure()}
                disabled={deleting() || !pendingDeleteProcedure()}
                class="sheet-action-btn sheet-action-btn-danger"
              >
                {deleting() ? "Deleting..." : "Delete"}
              </button>
            </div>
          </div>
        )}
      </ArkSheet>
    </div>
  );
}
