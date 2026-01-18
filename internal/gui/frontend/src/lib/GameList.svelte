<script>
  import { onMount, createEventDispatcher } from 'svelte'
  import { GetGames, ScanGames } from '../../wailsjs/go/main/App'

  const dispatch = createEventDispatcher()

  let games = []
  let search = ''
  let loading = true

  $: filteredGames = games.filter(g =>
    g.name.toLowerCase().includes(search.toLowerCase())
  )

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
</script>

<div class="game-list">
  <div class="header">
    <input
      type="text"
      placeholder="Search games..."
      bind:value={search}
    />
    <button on:click={rescan}>Rescan</button>
  </div>

  {#if loading}
    <div class="loading">Loading...</div>
  {:else if filteredGames.length === 0}
    <div class="empty">
      {#if search}
        No games matching "{search}"
      {:else}
        No games found. Try running 'spela scan' first.
      {/if}
    </div>
  {:else}
    <div class="list">
      {#each filteredGames as game}
        <button class="game-item" on:click={() => dispatch('select', game)}>
          <span class="name">{game.name}</span>
          <div class="badges">
            {#if game.dlls && game.dlls.length > 0}
              <span class="badge dlss">DLSS</span>
            {/if}
            {#if game.hasProfile}
              <span class="badge profile">Profile</span>
            {/if}
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .game-list {
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .header {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 1rem;
  }

  .header input {
    flex: 1;
    padding: 0.5rem 1rem;
    border: 1px solid var(--border-default);
    border-radius: 4px;
    background-color: var(--bg-secondary);
    color: var(--text-primary);
    font-size: 0.9rem;
  }

  .header input:focus {
    outline: none;
    border-color: var(--border-focus);
  }

  .header input::placeholder {
    color: var(--text-dim);
  }

  .header button {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
    cursor: pointer;
  }

  .header button:hover {
    filter: brightness(1.1);
  }

  .loading, .empty {
    color: var(--text-dim);
    text-align: center;
    padding: 2rem;
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    overflow-y: auto;
  }

  .game-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1rem;
    border: 1px solid var(--border-default);
    border-radius: 4px;
    background-color: var(--bg-secondary);
    color: var(--text-primary);
    cursor: pointer;
    text-align: left;
    transition: all 0.2s;
  }

  .game-item:hover {
    background-color: var(--bg-elevated);
    border-color: var(--accent-primary);
  }

  .name {
    font-weight: 500;
  }

  .badges {
    display: flex;
    gap: 0.5rem;
  }

  .badge {
    padding: 0.2rem 0.5rem;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 500;
  }

  .badge.dlss {
    background-color: var(--success);
    color: black;
  }

  .badge.profile {
    background-color: var(--accent-secondary);
    color: var(--color-ghost-white, #F5F5FD);
  }
</style>
