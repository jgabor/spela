<script>
  import { createEventDispatcher } from 'svelte'
  import { Destination, DestinationLabels } from './navState.js'

  export let active = Destination.Library

  const dispatch = createEventDispatcher()

  const items = [
    { dest: Destination.Library, hotkey: '1' },
    { dest: Destination.DLLCatalog, hotkey: '2' },
    { dest: Destination.Monitor, hotkey: '3' },
    { dest: Destination.Settings, hotkey: '4' }
  ]
</script>

<nav class="primary-nav" aria-label="Navigate">
  <div class="title">Navigate</div>
  {#each items as item}
    <button
      type="button"
      class="nav-item"
      class:active={active === item.dest}
      on:click={() => dispatch('select', { destination: item.dest })}
    >
      <span class="hotkey">[{item.hotkey}]</span>
      <span class="label">{DestinationLabels[item.dest]}</span>
    </button>
  {/each}
</nav>

<style>
  .primary-nav {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    padding: 0.75rem;
    border-right: 1px solid var(--border-default);
    min-width: 10rem;
    background: var(--bg-primary);
  }
  .title {
    font-size: 0.7rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--accent-primary);
    margin-bottom: 0.5rem;
  }
  .nav-item {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    width: 100%;
    text-align: left;
    padding: 0.4rem 0.5rem;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    font-family: inherit;
  }
  .nav-item:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }
  .nav-item.active {
    color: var(--text-primary);
    border-left: 2px solid var(--accent-focus);
    padding-left: calc(0.5rem - 2px);
  }
  .hotkey {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--accent-primary);
  }
</style>
