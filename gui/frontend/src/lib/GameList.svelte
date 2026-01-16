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
    border: 1px solid #3d4f5f;
    border-radius: 4px;
    background-color: #232f3e;
    color: #e0e0e0;
    font-size: 0.9rem;
  }

  .header input::placeholder {
    color: #8899a6;
  }

  .header button {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    background-color: #3d8bff;
    color: white;
    cursor: pointer;
  }

  .loading, .empty {
    color: #8899a6;
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
    border: 1px solid #3d4f5f;
    border-radius: 4px;
    background-color: #232f3e;
    color: #e0e0e0;
    cursor: pointer;
    text-align: left;
    transition: all 0.2s;
  }

  .game-item:hover {
    background-color: #2d3f4f;
    border-color: #3d8bff;
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
    background-color: #76b900;
    color: black;
  }

  .badge.profile {
    background-color: #3d8bff;
    color: white;
  }
</style>
