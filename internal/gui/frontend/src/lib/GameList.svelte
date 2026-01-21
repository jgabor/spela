<script>
  import { onMount, createEventDispatcher } from 'svelte'
  import { GetGames, ScanGames, UpdateDLLs } from '../../wailsjs/go/gui/App'
  import Dropdown from './Dropdown.svelte'

  export let selectedGame = null
  export let defaultProfileSelected = false

  const dispatch = createEventDispatcher()

  let games = []
  let search = ''
  let loading = true
  let sortMode = 'name-asc'
  let filterDLLs = false
  let filterProfile = false
  let selectMode = false
  let selected = new Set()
  let batchUpdating = false
  let batchMessage = ''
  let messageTimer

  const sortModes = [
    { value: 'name-asc', label: 'Name A-Z' },
    { value: 'name-desc', label: 'Name Z-A' },
    { value: 'dlls-first', label: 'DLLs first' },
    { value: 'profile-first', label: 'Profile first' }
  ]

  $: filteredGames = applyFiltersAndSort(games, search, filterDLLs, filterProfile, sortMode)
  $: showDefaultProfile = !selectMode && !hasActiveFilters

  function applyFiltersAndSort(list, searchQuery, dllFilter, profileFilter, sort) {
    let filtered = list.filter(g => {
      if (searchQuery && !g.name.toLowerCase().includes(searchQuery.toLowerCase())) {
        return false
      }
      if (dllFilter && (!g.dlls || g.dlls.length === 0)) {
        return false
      }
      if (profileFilter && !g.hasProfile) {
        return false
      }
      return true
    })
    return sortGames(filtered, sort)
  }

  function sortGames(list, sort) {
    const sorted = [...list]
    switch (sort) {
      case 'name-asc':
        sorted.sort((a, b) => a.name.localeCompare(b.name))
        break
      case 'name-desc':
        sorted.sort((a, b) => b.name.localeCompare(a.name))
        break
      case 'dlls-first':
        sorted.sort((a, b) => {
          const aHas = a.dlls && a.dlls.length > 0
          const bHas = b.dlls && b.dlls.length > 0
          if (aHas !== bHas) return bHas ? 1 : -1
          return a.name.localeCompare(b.name)
        })
        break
      case 'profile-first':
        sorted.sort((a, b) => {
          if (a.hasProfile !== b.hasProfile) return b.hasProfile ? 1 : -1
          return a.name.localeCompare(b.name)
        })
        break
    }
    return sorted
  }

  onMount(async () => {
    await loadGames()
  })

  async function loadGames() {
    loading = true
    try {
      games = await GetGames() || []
    } catch (e) {
      console.error('Failed to load games:', e)
    }
    loading = false
  }

  export async function refreshGames() {
    await loadGames()
  }

  async function rescan() {
    loading = true
    try {
      await ScanGames()
      games = await GetGames() || []
    } catch (e) {
      console.error('Failed to scan games:', e)
    }
    loading = false
  }

  function clearFilters() {
    search = ''
    filterDLLs = false
    filterProfile = false
    sortMode = 'name-asc'
  }

  function toggleSelectMode() {
    selectMode = !selectMode
    if (!selectMode) {
      selected = new Set()
    }
  }

  function toggleSelection(appId) {
    if (selected.has(appId)) {
      selected.delete(appId)
    } else {
      selected.add(appId)
    }
    selected = new Set(selected)
  }

  function selectAll() {
    filteredGames.forEach(game => {
      selected.add(game.appId)
    })
    selected = new Set(selected)
  }

  function selectNone() {
    selected = new Set()
  }

  function clearMessageAfter(delay) {
    if (messageTimer) {
      clearTimeout(messageTimer)
    }
    messageTimer = setTimeout(() => {
      batchMessage = ''
      messageTimer = null
    }, delay)
  }

  async function batchUpdateDLLs() {
    const gamesWithDLLs = filteredGames.filter(g => selected.has(g.appId) && g.dlls && g.dlls.length > 0)
    if (gamesWithDLLs.length === 0) {
      batchMessage = 'No selected games have DLLs to update'
      clearMessageAfter(3000)
      return
    }

    batchUpdating = true
    batchMessage = `Updating DLLs for ${gamesWithDLLs.length} games...`
    let successCount = 0
    let failCount = 0

    for (const g of gamesWithDLLs) {
      try {
        await UpdateDLLs(g.appId)
        successCount++
      } catch (e) {
        console.error(`Failed to update DLLs for ${g.name}:`, e)
        failCount++
      }
    }

    if (failCount > 0) {
      batchMessage = `Updated ${successCount} games, ${failCount} failed`
    } else {
      batchMessage = `Updated DLLs for ${successCount} games`
    }
    clearMessageAfter(5000)
    batchUpdating = false
    selected = new Set()
    selectMode = false
    await loadGames()
  }

  $: hasActiveFilters = search || filterDLLs || filterProfile || sortMode !== 'name-asc'
</script>

<div class="game-list">
  <div class="header">
    <input
      type="text"
      placeholder="Search games..."
      bind:value={search}
    />
    <div class="header-actions">
      <button on:click={rescan}>Rescan</button>
      <button class="select-btn" class:active={selectMode} on:click={toggleSelectMode}>
        {selectMode ? 'Cancel' : 'Select'}
      </button>
    </div>
  </div>

  {#if selectMode}
    <div class="batch-toolbar">
      <div class="batch-info">
        <span>{selected.size} selected</span>
        <button class="link-btn" on:click={selectAll}>Select all</button>
        <button class="link-btn" on:click={selectNone}>Select none</button>
      </div>
      <div class="batch-actions">
        <button
          class="batch-btn"
          on:click={batchUpdateDLLs}
          disabled={selected.size === 0 || batchUpdating}
        >
          {batchUpdating ? 'Updating...' : 'Update all DLLs'}
        </button>
      </div>
    </div>
    {#if batchMessage}
      <div class="batch-message">{batchMessage}</div>
    {/if}
  {:else}
    <div class="toolbar">
      <div class="filters">
        <label class="filter-toggle" class:active={filterDLLs}>
          <input type="checkbox" bind:checked={filterDLLs} />
          <span class="filter-label">● DLLs</span>
        </label>
        <label class="filter-toggle" class:active={filterProfile}>
          <input type="checkbox" bind:checked={filterProfile} />
          <span class="filter-label">◆ Profile</span>
        </label>
      </div>
      <div class="sort">
        <Dropdown
          bind:value={sortMode}
          options={sortModes}
        />
        {#if hasActiveFilters}
          <button class="clear-btn" on:click={clearFilters} title="Clear filters">✕</button>
        {/if}
      </div>
    </div>
  {/if}

  {#if loading}
    <div class="loading">Loading...</div>
  {:else}
    {#if showDefaultProfile}
      <button
        class="game-item default-profile"
        class:active={defaultProfileSelected}
        on:click={() => dispatch('selectDefaultProfile')}
      >
        <span class="name">Default profile</span>
        <div class="badges">
          <span class="badge default">Default</span>
        </div>
      </button>
    {/if}

    {#if filteredGames.length === 0}
      <div class="empty">
        {#if search || filterDLLs || filterProfile}
          No games matching filters
          <button class="clear-link" on:click={clearFilters}>Clear filters</button>
        {:else}
          No games found. Try running 'spela scan' first.
        {/if}
      </div>
    {:else}
      <div class="count">{filteredGames.length} game{filteredGames.length !== 1 ? 's' : ''}</div>
      <div class="list">
        {#each filteredGames as game}
        {#if selectMode}
          <label class="game-item selectable" class:selected={selected.has(game.appId)}>
            <input
              type="checkbox"
              checked={selected.has(game.appId)}
              on:change={() => toggleSelection(game.appId)}
            />
            <span class="name">{game.name}</span>
            <div class="badges">
              {#if game.dlls && game.dlls.length > 0}
                <span class="badge dlss">● DLLs</span>
              {/if}
              {#if game.hasProfile}
                <span class="badge profile">◆ Profile</span>
              {/if}
            </div>
          </label>
        {:else}
          <button
            class="game-item"
            class:active={selectedGame && selectedGame.appId === game.appId}
            on:click={() => dispatch('select', game)}
          >
            <span class="name">{game.name}</span>
            <div class="badges">
              {#if game.dlls && game.dlls.length > 0}
                <span class="badge dlss">● DLLs</span>
              {/if}
              {#if game.hasProfile}
                <span class="badge profile">◆ Profile</span>
              {/if}
            </div>
          </button>
        {/if}
      {/each}
      </div>
    {/if}
  {/if}
</div>


<style>
  .game-list {
    height: 100%;
    display: flex;
    flex-direction: column;
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .header {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    margin-bottom: 0.75rem;
  }

  .header input {
    flex: 1;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--border-default);
    border-radius: 6px;
    background-color: var(--bg-secondary);
    color: var(--text-primary);
    font-size: 0.8rem;
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .header input:focus {
    outline: none;
    border-color: var(--border-focus);
  }

  .header input::placeholder {
    color: var(--text-dim);
  }

  .header-actions {
    display: flex;
    gap: 0.5rem;
  }

  .header button {
    padding: 0.4rem 0.75rem;
    border: none;
    border-radius: 6px;
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
    cursor: pointer;
    font-size: 0.8rem;
    font-family: var(--font-ui, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  }

  .header button:hover {
    filter: brightness(1.1);
  }

  .select-btn {
    background-color: var(--bg-secondary);
    color: var(--text-primary);
  }

  .select-btn.active {
    background-color: var(--warning);
    color: black;
  }

  .game-item.active .badges .badge {
    background-color: rgba(255, 255, 255, 0.2);
    color: var(--color-ghost-white, #F5F5FD);
  }

  .batch-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem;
    background-color: var(--bg-secondary);
    border-radius: 6px;
    margin-bottom: 0.75rem;
  }

  .batch-info {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    font-size: 0.9rem;
  }

  .link-btn {
    background: none;
    border: none;
    color: var(--accent-secondary);
    cursor: pointer;
    text-decoration: underline;
    font-size: 0.85rem;
    padding: 0;
  }

  .link-btn:hover {
    color: var(--accent-primary);
  }

  .batch-actions {
    display: flex;
    gap: 0.5rem;
  }

  .batch-btn {
    padding: 0.4rem 0.75rem;
    border: none;
    border-radius: 4px;
    background-color: var(--success);
    color: black;
    cursor: pointer;
    font-size: 0.85rem;
  }

  .batch-btn:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .batch-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .batch-message {
    padding: 0.5rem;
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
    border-radius: 6px;
    text-align: center;
    margin-bottom: 0.75rem;
    font-size: 0.85rem;
  }

  .toolbar {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    margin-bottom: 0.6rem;
    gap: 0.6rem;
  }

  .filters {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .filter-toggle {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.25rem 0.5rem;
    border-radius: 6px;
    background-color: var(--bg-secondary);
    cursor: pointer;
    font-size: 0.7rem;
    transition: all 0.2s;
  }

  .filter-toggle input {
    display: none;
  }

  .filter-toggle .filter-label {
    color: var(--text-dim);
  }

  .filter-toggle.active {
    background-color: var(--accent-primary);
  }

  .filter-toggle.active .filter-label {
    color: var(--color-ghost-white, #F5F5FD);
  }

  .filter-toggle.active .filter-label::before {
    content: '> ';
  }

  .filter-toggle:hover {
    filter: brightness(1.1);
  }

  .sort {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
  }

  .clear-btn {
    padding: 0.25rem 0.5rem;
    border: none;
    border-radius: 4px;
    background-color: transparent;
    color: var(--text-dim);
    cursor: pointer;
    font-size: 0.8rem;
  }

  .clear-btn:hover {
    color: var(--text-primary);
  }

  .count {
    font-size: 0.65rem;
    color: var(--text-dim);
    margin-bottom: 0.4rem;
  }

  .loading, .empty {
    color: var(--text-dim);
    text-align: center;
    padding: 2rem;
  }

  .clear-link {
    display: block;
    margin-top: 0.5rem;
    background: none;
    border: none;
    color: var(--accent-secondary);
    cursor: pointer;
    text-decoration: underline;
  }

  .clear-link:hover {
    color: var(--accent-primary);
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    overflow-y: auto;
  }

  .game-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.6rem 0.85rem;
    border: 1px solid transparent;
    border-radius: 6px;
    background-color: transparent;
    color: var(--text-primary);
    cursor: pointer;
    text-align: left;
    font-size: 0.78rem;
    transition: all 0.2s;
  }

  .game-item.default-profile {
    border: 1px dashed var(--border-default);
    background-color: var(--bg-secondary);
  }

  .game-item:hover {
    background-color: var(--bg-secondary);
    border-color: var(--accent-primary);
  }

  .game-item.active {
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
  }

  .game-item.active .name {
    color: var(--color-ghost-white, #F5F5FD);
  }

  .game-item.active .badge.dlss,
  .game-item.active .badge.profile {
    background-color: rgba(255, 255, 255, 0.2);
    color: var(--color-ghost-white, #F5F5FD);
  }

  .game-item.selectable {
    cursor: pointer;
    letter-spacing: 0.02em;
    font-size: 0.82rem;
  }


  .game-item.selectable input[type="checkbox"] {
    width: 18px;
    height: 18px;
    accent-color: var(--accent-primary);
    margin-right: 0.5rem;
    flex-shrink: 0;
  }

  .game-item.selected {
    border-color: var(--accent-primary);
    background-color: var(--bg-elevated);
  }

  .name {
    font-weight: 500;
    color: inherit;
  }

  .badges {
    display: flex;
    gap: 0.5rem;
  }

  .badge {
    padding: 0.15rem 0.45rem;
    border-radius: 6px;
    font-size: 0.7rem;
    font-weight: 500;
    text-transform: none;
  }

  .badge.dlss {
    background-color: var(--accent-secondary);
    color: var(--color-ghost-white, #F5F5FD);
  }

  .badge.profile,
  .badge.default {
    background-color: var(--accent-secondary);
    color: var(--color-ghost-white, #F5F5FD);
  }
</style>
