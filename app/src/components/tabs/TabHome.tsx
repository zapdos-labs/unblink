import { For, Show, onMount, createSignal } from 'solid-js';
import { FiVideo, FiInfo, FiServer, FiEdit2, FiTrash2 } from 'solid-icons/fi';
import { nodes, nodesLoading, fetchNodes, setTab, relayFetch, type NodeService, type NodeInfo } from '../../shared';
import { ArkDialog } from '../../ark/ArkDialog';
import { ArkTabs } from '../../ark/ArkTabs';
import { Dialog } from '@ark-ui/solid/dialog';
import { toaster } from '../../ark/ArkToast';

interface ServiceWithNode {
  service: NodeService;
  nodeId: string;
}

function ServiceInfoDialog(props: { service: NodeService; nodeId: string }) {
  return (
    <ArkDialog
      trigger={(_, setOpen) => (
        <button
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            setOpen(true);
          }}
          class="px-3 py-1.5 rounded-lg border border-neu-750 bg-neu-800 hover:bg-neu-850 text-neu-300 text-xs font-medium transition-colors"
        >
          Detail
        </button>
      )}
      title="Service Details"
      description="View all information about this service"
    >
      <div class="mt-4 space-y-4">
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Service ID</label>
          <p class="mt-1 text-sm text-white break-all font-mono">{props.service.id}</p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Service Name</label>
          <p class="mt-1 text-sm text-white">{props.service.name || 'Unnamed Service'}</p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Type</label>
          <p class="mt-1">
            <span class="bg-neu-800 text-neu-300 text-xs font-medium px-2.5 py-0.5 rounded">
              {props.service.type}
            </span>
          </p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Node ID</label>
          <p class="mt-1 text-sm text-white break-all font-mono">{props.nodeId}</p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Address</label>
          <p class="mt-1 text-sm text-white font-mono">{props.service.addr}:{props.service.port}{props.service.path}</p>
        </div>
      </div>
    </ArkDialog>
  );
}

function NodeEditDialog(props: { node: NodeInfo }) {
  const [name, setName] = createSignal(props.node.name || '');

  onMount(() => {
    setName(props.node.name || '');
  });

  const handleSave = async () => {
    const newName = name().trim();

    if (!newName) {
      return;
    }

    if (newName.length > 255) {
      return;
    }

    toaster.promise(async () => {
      const response = await relayFetch(`/node/${props.node.id}/name`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name: newName }),
      });

      if (response.ok) {
        await fetchNodes();
      } else {
        throw new Error('Failed to update node name');
      }
    }, {
      loading: {
        title: 'Saving...',
        description: 'Your node is being updated.',
      },
      success: {
        title: 'Success!',
        description: 'Node has been updated successfully.',
      },
      error: {
        title: 'Failed',
        description: 'There was an error updating your node. Please try again.',
      },
    });
  };

  return (
    <ArkDialog
      trigger={(_, setOpen) => (
        <button
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            setOpen(true);
          }}
          class="px-3 py-1.5 rounded-lg border border-neu-750 bg-neu-800 hover:bg-neu-850 text-neu-300 text-xs font-medium transition-colors flex items-center gap-1.5"
        >
          <FiEdit2 class="w-3 h-3" />
          Edit
        </button>
      )}
      title="Edit Node Name"
      description="Change the display name for this node"
    >
      <div class="mt-4 space-y-4">
        <div>
          <label for="node-name" class="text-sm font-medium text-neu-300">Node Name</label>
          <input
            id="node-name"
            type="text"
            value={name()}
            onInput={(e) => setName(e.currentTarget.value)}
            placeholder="Enter node name"
            class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500"
          />
        </div>
        <div class="flex justify-end pt-4">
          <Dialog.CloseTrigger>
            <button
              onClick={handleSave}
              class="btn-primary"
            >
              Save Node
            </button>
          </Dialog.CloseTrigger>
        </div>
      </div>
    </ArkDialog>
  );
}

function NodeDeleteDialog(props: { node: NodeInfo }) {
  const handleDelete = async () => {
    toaster.promise(async () => {
      const response = await relayFetch(`/node/${props.node.id}/delete`, {
        method: 'DELETE',
      });

      if (response.ok) {
        await fetchNodes();
      } else {
        throw new Error('Failed to delete node');
      }
    }, {
      loading: {
        title: 'Deleting...',
        description: 'Your node is being deleted.',
      },
      success: {
        title: 'Success!',
        description: 'Node has been deleted successfully.',
      },
      error: {
        title: 'Failed',
        description: 'There was an error deleting your node. Please try again.',
      },
    });
  };

  return (
    <ArkDialog
      trigger={(_, setOpen) => (
        <button
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            setOpen(true);
          }}
          class="px-3 py-1.5 rounded-lg border border-neu-750 bg-neu-800 hover:bg-neu-850 text-neu-300 text-xs font-medium transition-colors flex items-center gap-1.5"
        >
          <FiTrash2 class="w-3 h-3" />
          Delete
        </button>
      )}
      title="Delete Node"
      description={`Are you sure you want to delete "${props.node.name || 'Unnamed Node'}"? This action cannot be undone.`}
    >
      <div class="flex justify-end pt-4">
        <Dialog.CloseTrigger>
          <button
            onClick={handleDelete}
            class="btn-danger"
          >
            Delete Node
          </button>
        </Dialog.CloseTrigger>
      </div>
    </ArkDialog>
  );
}

function NodeInfoDialog(props: { node: NodeInfo }) {
  return (
    <ArkDialog
      trigger={(_, setOpen) => (
        <button
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            setOpen(true);
          }}
          class="px-3 py-1.5 rounded-lg border border-neu-750 bg-neu-800 hover:bg-neu-850 text-neu-300 text-xs font-medium transition-colors"
        >
          Detail
        </button>
      )}
      title="Node Details"
      description="View all information about this node"
    >
      <div class="mt-4 space-y-4">
        <Show when={props.node.name}>
          <div>
            <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Name</label>
            <p class="mt-1 text-sm text-white">{props.node.name}</p>
          </div>
        </Show>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Node ID</label>
          <p class="mt-1 text-sm text-white break-all font-mono">{props.node.id}</p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Status</label>
          <p class="mt-1">
            <span class={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded text-xs font-medium ${props.node.status === 'online'
              ? 'bg-green-900/30 text-green-400'
              : 'bg-neu-800 text-neu-400'
              }`}>
              <span class={`w-1.5 h-1.5 rounded-full ${props.node.status === 'online' ? 'bg-green-500' : 'bg-neu-500'
                }`}></span>
              <span class="capitalize">{props.node.status}</span>
            </span>
          </p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Services</label>
          <p class="mt-1 text-sm text-white">{props.node.services.length} service(s)</p>
        </div>
        <Show when={props.node.lastConnectedAt}>
          <div>
            <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Last Connected</label>
            <p class="mt-1 text-sm text-neu-300">
              {new Date(props.node.lastConnectedAt!).toLocaleString()}
            </p>
          </div>
        </Show>
      </div>
    </ArkDialog>
  );
}

function ServicesView() {
  // Flatten all services from all nodes
  const allServices = (): ServiceWithNode[] => {
    const result: ServiceWithNode[] = [];
    for (const node of nodes()) {
      for (const service of node.services) {
        result.push({ service, nodeId: node.id });
      }
    }
    return result;
  };

  const handleServiceClick = (nodeId: string, serviceId: string, name: string) => {
    setTab({ type: 'view', nodeId, serviceId, name });
  };

  return (
    <Show
      when={allServices().length > 0}
      fallback={
        <div class="h-full flex items-center justify-center text-neu-500">
          <div class="text-center">
            <FiVideo class="mx-auto mb-4 w-12 h-12" />
            <p>No services found</p>
            <p>Connect a node with services to get started</p>
          </div>
        </div>
      }
    >
      <div class="relative overflow-x-auto h-full">
        <table class="w-full text-sm text-left text-neu-400">
          <thead class="text-neu-400 font-normal">
            <tr>
              <th scope="col" class="px-6 py-3 font-medium">
                Service Name
              </th>
              <th scope="col" class="px-6 py-3 font-medium">
                Type
              </th>
              <th scope="col" class="px-6 py-3 font-medium">
                Node ID
              </th>
              <th scope="col" class="px-6 py-3 font-medium">
                Address
              </th>
              <th scope="col" class="px-6 py-3 font-medium">
                Actions
              </th>
            </tr>
          </thead>
          <tbody>
            <For each={allServices()}>
              {({ service, nodeId }) => (
                <tr
                  class="border-b bg-neu-900 border-neu-800 cursor-pointer hover:bg-neu-850 transition-colors"
                  onClick={() => handleServiceClick(nodeId, service.id, service.name)}
                >
                  <td class="px-6 py-4 font-medium text-white">
                    {service.name || 'Unnamed Service'}
                  </td>
                  <td class="px-6 py-4">
                    <span class="bg-neu-800 text-neu-300 text-xs font-medium px-2.5 py-0.5 rounded whitespace-nowrap">
                      {service.type}
                    </span>
                  </td>
                  <td class="px-6 py-4 max-w-[20vw]">
                    <span class="line-clamp-1 break-all text-xs">{nodeId}</span>
                  </td>
                  <td class="px-6 py-4">
                    <span class="text-xs">{service.addr}:{service.port}{service.path}</span>
                  </td>
                  <td class="px-6 py-4">
                    <ServiceInfoDialog service={service} nodeId={nodeId} />
                  </td>
                </tr>
              )}
            </For>
          </tbody>
        </table>
      </div>
    </Show>
  );
}

function NodesView() {
  // Sort nodes: online first, then by most recent connection
  const sortedNodes = () => {
    return [...nodes()].sort((a, b) => {
      // Online nodes come first
      if (a.status === 'online' && b.status !== 'online') return -1;
      if (a.status !== 'online' && b.status === 'online') return 1;

      // Within same status, sort by last connected time (most recent first)
      const aTime = a.lastConnectedAt ? new Date(a.lastConnectedAt).getTime() : 0;
      const bTime = b.lastConnectedAt ? new Date(b.lastConnectedAt).getTime() : 0;
      return bTime - aTime;
    });
  };

  return (
    <Show
      when={nodes().length > 0}
      fallback={
        <div class="h-full flex items-center justify-center text-neu-500">
          <div class="text-center">
            <FiServer class="mx-auto mb-4 w-12 h-12" />
            <p>No nodes found</p>
            <p>Connect a node to get started</p>
          </div>
        </div>
      }
    >
      <div class="relative overflow-x-auto h-full">
        <table class="w-full text-sm text-left text-neu-400">
          <thead class="text-neu-400 font-normal">
            <tr>
              <th scope="col" class="px-6 py-3 font-medium">
                Name
              </th>
              <th scope="col" class="px-6 py-3 font-medium">
                Node ID
              </th>
              <th scope="col" class="px-6 py-3 font-medium">
                Status
              </th>
              <th scope="col" class="px-6 py-3 font-medium">
                Services
              </th>
              <th scope="col" class="px-6 py-3 font-medium">
                Actions
              </th>
            </tr>
          </thead>
          <tbody>
            <For each={sortedNodes()}>
              {(node) => (
                <tr class="border-b bg-neu-900 border-neu-800 hover:bg-neu-850 transition-colors">
                  <td class="px-6 py-4">
                    <p class="font-medium text-white text-sm">{node.name || 'Unnamed Node'}</p>
                  </td>
                  <td class="px-6 py-4 max-w-[25vw]">
                    <p class="text-xs text-neu-400 font-mono line-clamp-1 break-all">{node.id}</p>
                  </td>
                  <td class="px-6 py-4">
                    <span class="inline-flex items-center gap-1.5">
                      <span class={`w-2 h-2 rounded-full ${node.status === 'online' ? 'bg-green-500' : 'bg-neu-500'}`}></span>
                      <span class="text-xs text-neu-300 capitalize">{node.status}</span>
                    </span>
                  </td>
                  <td class="px-6 py-4">
                    <Show
                      when={node.status === 'online'}
                      fallback={<span class="text-xs text-neu-500">-</span>}
                    >
                      <span class="text-xs text-neu-300">{node.services.length} service(s)</span>
                    </Show>
                  </td>
                  <td class="px-6 py-4">
                    <div class="flex items-center gap-2">
                      <NodeEditDialog node={node} />
                      <NodeDeleteDialog node={node} />
                      <NodeInfoDialog node={node} />
                    </div>
                  </td>
                </tr>
              )}
            </For>
          </tbody>
        </table>
      </div>
    </Show>
  );
}

export default function HomeContent() {
  const [activeTab, setActiveTab] = createSignal('services');

  onMount(fetchNodes);

  return (
    <Show
      when={!nodesLoading()}
      fallback={
        <div class="h-full flex items-center justify-center">
          <div class="text-neu-500">Loading nodes...</div>
        </div>
      }
    >
      <ArkTabs
        items={[
          {
            value: 'services',
            label: 'Services',
            icon: <FiVideo size={16} />,
            content: <ServicesView />
          },
          {
            value: 'nodes',
            label: 'Nodes',
            icon: <FiServer size={16} />,
            content: <NodesView />
          }
        ]}
        value={activeTab()}
        onValueChange={setActiveTab}
      />
    </Show>
  );
}
