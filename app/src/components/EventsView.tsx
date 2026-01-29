import { createSignal, onMount, Show, For } from 'solid-js'
import { eventClient } from '../lib/rpc'
import { services } from '../shared'
import { toaster } from '../ark/ArkToast'
import type { Event } from '@/gen/service/v1/event_pb'
import { FiChevronLeft, FiChevronRight, FiRefreshCw, FiX } from 'solid-icons/fi'

const EVENTS_PER_PAGE = 20

interface EventsViewProps {
  nodeId: string
}

interface CellContent {
  title: string
  content: string
  x: number
  y: number
}

export default function EventsView(props: EventsViewProps) {
  const [events, setEvents] = createSignal<Event[]>([])
  const [totalCount, setTotalCount] = createSignal(0)
  const [loading, setLoading] = createSignal(true)
  const [currentPage, setCurrentPage] = createSignal(0)
  const [selectedCell, setSelectedCell] = createSignal<{ row: number; col: number } | null>(null)
  const [modalContent, setModalContent] = createSignal<CellContent | null>(null)

  const loadEvents = async (page: number = 0) => {
    setLoading(true)
    try {
      const res = await eventClient.listEventsByNodeId({
        nodeId: props.nodeId,
        pageSize: EVENTS_PER_PAGE,
        pageOffset: page * EVENTS_PER_PAGE,
      })
      console.log('res.events', res.events)
      setEvents(res.events)
      setTotalCount(res.totalCount)
      setCurrentPage(page)
    } catch (error) {
      console.error('Failed to load events:', error)
      toaster.create({
        title: 'Failed to load events',
        description: error instanceof Error ? error.message : String(error),
        type: 'error',
      })
    } finally {
      setLoading(false)
    }
  }

  onMount(() => {
    loadEvents()
  })

  const getServiceName = (serviceId: string) => {
    const service = services().find((s) => s.id === serviceId)
    return service?.name || serviceId
  }

  const totalPages = () => Math.ceil(totalCount() / EVENTS_PER_PAGE)
  const hasPrev = () => currentPage() > 0
  const hasNext = () => (currentPage() + 1) < totalPages()

  const formatDate = (event: Event) => {
    if (event.createdAt) {
      const seconds = Number(event.createdAt.seconds ?? 0n)
      const nanos = event.createdAt.nanos ?? 0
      const date = new Date(seconds * 1000 + nanos / 1_000_000)
      return date.toLocaleString()
    }
    return 'N/A'
  }

  const formatPayload = (payload: any) => {
    if (!payload) return 'null'
    // Struct payload is already a plain object, just stringify it
    return JSON.stringify(payload)
  }

  const handlePrev = () => {
    if (hasPrev()) {
      loadEvents(currentPage() - 1)
    }
  }

  const handleNext = () => {
    if (hasNext()) {
      loadEvents(currentPage() + 1)
    }
  }

  const handleCellClick = (rowIndex: number, colIndex: number) => {
    setSelectedCell({ row: rowIndex, col: colIndex })
  }

  const handleCellDoubleClick = (event: MouseEvent, title: string, content: string) => {
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
    const modalWidth = 400
    const modalHeight = 384 // max-h-96 = 384px

    // Position modal: prefer right of cell, fall back to left if not enough space
    let x = rect.right + 8
    if (x + modalWidth > window.innerWidth) {
      // Not enough space on right, try left
      x = rect.left - modalWidth - 8
      if (x < 0) {
        // Not enough space on left either, center horizontally
        x = Math.max(8, (window.innerWidth - modalWidth) / 2)
      }
    }

    // Position modal vertically: align with top of cell, adjust if needed
    let y = rect.top
    if (y + modalHeight > window.innerHeight) {
      // Not enough space below, align with bottom
      y = Math.max(8, window.innerHeight - modalHeight - 8)
    }

    setModalContent({
      title,
      content,
      x,
      y,
    })
  }

  const closeModal = () => {
    setModalContent(null)
  }

  const getCellClass = (rowIndex: number, colIndex: number) => {
    const selected = selectedCell()
    const isSelected = selected && selected.row === rowIndex && selected.col === colIndex
    return isSelected ? 'ring-2 ring-violet-500 ring-inset' : ''
  }

  return (
    <div class="flex flex-col h-full bg-neu-900">
      {/* Header */}
      <div class="flex items-center justify-between px-6 py-4 border-b border-neu-800">
        <div>
          <h1 class="text-xl font-semibold text-white">Events</h1>
          <p class="text-sm text-neu-400 mt-1">
            {totalCount()} event{totalCount() !== 1 ? 's' : ''} across {services().length} service{services().length !== 1 ? 's' : ''}
          </p>
        </div>
        <button
          onClick={() => loadEvents(currentPage())}
          class="drop-shadow-xl px-3 py-1.5 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 flex items-center space-x-2 text-sm text-white disabled:opacity-50 disabled:cursor-not-allowed"
          disabled={loading()}
        >
          <FiRefreshCw class={`w-4 h-4 ${loading() ? 'animate-spin' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {/* Table */}
      <div class="flex-1 overflow-auto">
        <Show
          when={!loading() && events().length > 0}
          fallback={
            <div class="flex items-center justify-center h-full">
              <div class="text-center">
                <p class="text-neu-400">
                  {loading() ? 'Loading events...' : 'No events found'}
                </p>
              </div>
            </div>
          }
        >
          <table class="w-full border-collapse table-fixed select-none">
            <thead class="sticky top-0 bg-neu-850">
              <tr>
                <th class="text-left px-6 py-3 text-sm font-medium text-neu-400 border-b border-neu-800 w-64">
                  <div class="line-clamp-1 break-all">ID</div>
                </th>
                <th class="text-left px-6 py-3 text-sm font-medium text-neu-400 border-b border-neu-800 w-48">
                  <div class="line-clamp-1 break-all">Service</div>
                </th>
                <th class="text-left px-6 py-3 text-sm font-medium text-neu-400 border-b border-neu-800 w-96">
                  <div class="line-clamp-1 break-all">Payload</div>
                </th>
                <th class="text-left px-6 py-3 text-sm font-medium text-neu-400 border-b border-neu-800 w-48">
                  <div class="line-clamp-1 break-all">Created At</div>
                </th>
              </tr>
            </thead>
            <tbody>
              <For each={events()}>
                {(event, index) => (
                  <tr class="hover:bg-neu-800/50 transition-colors border-b border-neu-800">
                    <td
                      class={`px-6 py-3 text-sm text-white font-mono cursor-pointer ${getCellClass(index(), 0)}`}
                      onClick={() => handleCellClick(index(), 0)}
                      onDblClick={(e) => handleCellDoubleClick(e, 'Event ID', event.id)}
                    >
                      <div class="line-clamp-1 break-all">{event.id}</div>
                    </td>
                    <td
                      class={`px-6 py-3 text-sm text-neu-300 cursor-pointer ${getCellClass(index(), 1)}`}
                      onClick={() => handleCellClick(index(), 1)}
                      onDblClick={(e) => handleCellDoubleClick(e, 'Service', getServiceName(event.serviceId))}
                    >
                      <div class="line-clamp-1 break-all">{getServiceName(event.serviceId)}</div>
                    </td>
                    <td
                      class={`px-6 py-3 text-sm text-neu-400 font-mono cursor-pointer ${getCellClass(index(), 2)}`}
                      onClick={() => handleCellClick(index(), 2)}
                      onDblClick={(e) => handleCellDoubleClick(e, 'Payload', formatPayload(event.payload))}
                    >
                      <div class="line-clamp-1 break-all">
                        {formatPayload(event.payload)}
                      </div>
                    </td>
                    <td
                      class={`px-6 py-3 text-sm text-neu-400 cursor-pointer ${getCellClass(index(), 3)}`}
                      onClick={() => handleCellClick(index(), 3)}
                      onDblClick={(e) => handleCellDoubleClick(e, 'Created At', formatDate(event))}
                    >
                      <div class="line-clamp-1 break-all">{formatDate(event)}</div>
                    </td>
                  </tr>
                )}
              </For>
            </tbody>
          </table>
        </Show>
      </div>

      {/* Pagination */}
      <Show when={totalPages() > 1}>
        <div class="flex items-center justify-between px-6 py-4 border-t border-neu-800 bg-neu-850">
          <div class="text-sm text-neu-400">
            Page {currentPage() + 1} of {totalPages()}
          </div>
          <div class="flex items-center space-x-2">
            <button
              onClick={handlePrev}
              disabled={!hasPrev()}
              class="p-2 rounded-lg bg-neu-800 text-white disabled:opacity-50 disabled:cursor-not-allowed hover:bg-neu-700 transition-colors"
            >
              <FiChevronLeft class="w-4 h-4" />
            </button>
            <button
              onClick={handleNext}
              disabled={!hasNext()}
              class="p-2 rounded-lg bg-neu-800 text-white disabled:opacity-50 disabled:cursor-not-allowed hover:bg-neu-700 transition-colors"
            >
              <FiChevronRight class="w-4 h-4" />
            </button>
          </div>
        </div>
      </Show>

      {/* Modal for full content */}
      <Show when={modalContent()}>
        {(content) => (
          <>
            {/* Backdrop */}
            <div
              class="fixed inset-0 bg-black/50 z-40"
              onClick={closeModal}
            />
            {/* Modal */}
            <div
              class="fixed z-50 bg-neu-850 border border-neu-700 rounded-lg shadow-xl w-[400px] max-h-96 flex flex-col"
              style={{
                left: `${content().x}px`,
                top: `${content().y}px`,
              }}
            >
              {/* Header */}
              <div class="flex items-center justify-between px-4 py-3 border-b border-neu-700">
                <h3 class="text-sm font-semibold text-white">{content().title}</h3>
                <button
                  onClick={closeModal}
                  class="p-1 rounded hover:bg-neu-800 text-neu-400 hover:text-white transition-colors"
                >
                  <FiX class="w-4 h-4" />
                </button>
              </div>
              {/* Content */}
              <div class="flex-1 overflow-auto p-4">
                <pre class="text-sm text-neu-300 font-mono whitespace-pre-wrap break-all">
                  {content().content}
                </pre>
              </div>
            </div>
          </>
        )}
      </Show>
    </div>
  )
}
