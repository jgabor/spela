<script>
  import { createEventDispatcher } from 'svelte'
  import GameList from './GameList.svelte'
  import {
    Aspect,
    AspectLabels,
    ProfileSubsystemLabels,
    Destination,
    SettingsSectionLabels,
    MonitorSectionLabels
  } from './navState.js'

  export let desktop
  export let destination = Destination.Library
  export let selectedGame = null
  export let scopeGlobal = true
  export let aspect = Aspect.Profile
  export let subsystem = 0
  export let dllSection = 0
  export let monitorSection = 0
  export let settingsSection = 0

  const dispatch = createEventDispatcher()

  let gameListComponent

  export function focusSearch() {
    gameListComponent?.focusSearch?.()
  }
</script>

<nav class="context-nav" aria-label="Context">
  <div class="title">{destination === Destination.Library ? 'Scope' : 'Sections'}</div>
  {#if destination === Destination.Library}
    <GameList
      bind:this={gameListComponent}
      {desktop}
      {selectedGame}
      defaultProfileSelected={scopeGlobal}
      on:select={e => dispatch('selectGame', e.detail)}
      on:selectDefaultProfile={() => dispatch('selectGlobal')}
    />
    <div class="aspect-tabs" role="tablist">
      {#each AspectLabels as label, index}
        {#if !scopeGlobal || index === Aspect.Profile}
          <button
            type="button"
            role="tab"
            class:active={aspect === index}
            disabled={scopeGlobal && index !== Aspect.Profile}
            on:click={() => dispatch('selectAspect', { aspect: index })}
          >{index + 1} {label}</button>
        {/if}
      {/each}
    </div>
    {#if aspect === Aspect.Profile}
      <div class="subsystem-list">
        <div class="sub-title">Subsystem</div>
        {#each ProfileSubsystemLabels as label, index}
          <button
            type="button"
            class:active={subsystem === index}
            on:click={() => dispatch('selectSubsystem', { subsystem: index })}
          >{label}</button>
        {/each}
      </div>
    {/if}
  {:else}
    <div class="section-list">
      {#if destination === Destination.DLLCatalog}
        <button type="button" class:active={dllSection === 0} on:click={() => dispatch('dllSection', { section: 0 })}>Library</button>
        <button type="button" class:active={dllSection === 1} on:click={() => dispatch('dllSection', { section: 1 })}>Deployment</button>
      {:else if destination === Destination.Monitor}
        {#each MonitorSectionLabels as label, index}
          <button type="button" class:active={monitorSection === index} on:click={() => dispatch('monitorSection', { section: index })}>{label}</button>
        {/each}
      {:else}
        {#each SettingsSectionLabels as label, index}
          <button type="button" class:active={settingsSection === index} on:click={() => dispatch('settingsSection', { section: index })}>{label}</button>
        {/each}
      {/if}
    </div>
  {/if}
</nav>

<style>
  .context-nav {
    display: flex;
    flex-direction: column;
    min-width: 14rem;
    max-width: 18rem;
    border-right: 1px solid var(--border-default);
    background: var(--bg-primary);
    overflow-y: auto;
  }
  .title, .sub-title {
    font-size: 0.7rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-dim);
    padding: 0.5rem 0.75rem 0.25rem;
  }
  .aspect-tabs {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.5rem 0.75rem;
    border-top: 1px solid var(--border-default);
  }
  .aspect-tabs button,
  .section-list button,
  .subsystem-list button {
    text-align: left;
    padding: 0.35rem 0.5rem;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    font-family: inherit;
    font-size: 0.8rem;
  }
  .aspect-tabs button.active,
  .section-list button.active,
  .subsystem-list button.active {
    color: var(--accent-focus);
  }
  .subsystem-list {
    border-top: 1px solid var(--border-default);
    padding-bottom: 0.5rem;
  }
  .section-list {
    display: flex;
    flex-direction: column;
    padding: 0.5rem;
    gap: 0.15rem;
  }
</style>
