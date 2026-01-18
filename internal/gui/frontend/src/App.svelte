<script>
  import { onMount } from 'svelte'
  import GameList from './lib/GameList.svelte'
  import GameDetail from './lib/GameDetail.svelte'
  import Monitor from './lib/Monitor.svelte'

  let currentView = 'games'
  let selectedGame = null
  let theme = 'dark'

  onMount(() => {
    const stored = localStorage.getItem('spela-theme')
    if (stored) {
      theme = stored
      document.documentElement.setAttribute('data-theme', theme)
    }
  })

  function toggleTheme() {
    theme = theme === 'dark' ? 'light' : 'dark'
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('spela-theme', theme)
  }

  function selectGame(game) {
    selectedGame = game
    currentView = 'detail'
  }

  function goBack() {
    if (currentView === 'detail') {
      selectedGame = null
      currentView = 'games'
    }
  }
</script>

<main>
  <nav>
    <div class="nav-left">
      <button class:active={currentView === 'games'} on:click={() => currentView = 'games'}>
        Games
      </button>
      <button class:active={currentView === 'monitor'} on:click={() => currentView = 'monitor'}>
        Monitor
      </button>
    </div>
    <button class="theme-toggle" on:click={toggleTheme} title="Toggle theme">
      {theme === 'dark' ? '☀️' : '🌙'}
    </button>
  </nav>

  <div class="content">
    {#if currentView === 'games'}
      <GameList on:select={e => selectGame(e.detail)} />
    {:else if currentView === 'detail' && selectedGame}
      <GameDetail game={selectedGame} on:back={goBack} />
    {:else if currentView === 'monitor'}
      <Monitor />
    {/if}
  </div>
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  nav {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    background-color: var(--bg-secondary);
    border-bottom: 1px solid var(--border-default);
  }

  .nav-left {
    display: flex;
    gap: 0.5rem;
  }

  nav button {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    background-color: transparent;
    color: var(--text-dim);
    cursor: pointer;
    font-size: 0.9rem;
    transition: all 0.2s;
  }

  nav button:hover {
    background-color: var(--border-default);
    color: var(--text-primary);
  }

  nav button.active {
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
  }

  .theme-toggle {
    font-size: 1.2rem;
    padding: 0.5rem;
  }

  .content {
    flex: 1;
    overflow: auto;
    padding: 1rem;
  }
</style>
