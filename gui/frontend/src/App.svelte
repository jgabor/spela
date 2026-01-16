<script>
  import { onMount } from 'svelte'
  import GameList from './lib/GameList.svelte'
  import GameDetail from './lib/GameDetail.svelte'
  import Monitor from './lib/Monitor.svelte'

  let currentView = 'games'
  let selectedGame = null

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
    <button class:active={currentView === 'games'} on:click={() => currentView = 'games'}>
      Games
    </button>
    <button class:active={currentView === 'monitor'} on:click={() => currentView = 'monitor'}>
      Monitor
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
    gap: 0.5rem;
    padding: 1rem;
    background-color: #232f3e;
    border-bottom: 1px solid #3d4f5f;
  }

  nav button {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    background-color: transparent;
    color: #8899a6;
    cursor: pointer;
    font-size: 0.9rem;
    transition: all 0.2s;
  }

  nav button:hover {
    background-color: #3d4f5f;
    color: #e0e0e0;
  }

  nav button.active {
    background-color: #3d8bff;
    color: white;
  }

  .content {
    flex: 1;
    overflow: auto;
    padding: 1rem;
  }
</style>
