<script>
  import { createEventDispatcher } from 'svelte'

  export let value = ''
  export let options = []
  export let placeholder = 'Select...'

  const dispatch = createEventDispatcher()

  let open = false
  let buttonElement

  $: selectedOption = options.find(o => o.value === value)
  $: displayText = selectedOption ? selectedOption.label : placeholder

  function toggle() {
    open = !open
  }

  function select(option) {
    value = option.value
    open = false
    dispatch('change', option.value)
  }

  function handleClickOutside(event) {
    if (buttonElement && !buttonElement.contains(event.target)) {
      open = false
    }
  }

  function handleKeydown(event) {
    if (event.key === 'Escape') {
      open = false
    }
  }
</script>

<svelte:window on:click={handleClickOutside} on:keydown={handleKeydown} />

<div class="dropdown" bind:this={buttonElement}>
  <button type="button" class="trigger" on:click={toggle}>
    <span>{displayText}</span>
    <span class="arrow">{open ? '▲' : '▼'}</span>
  </button>
  {#if open}
    <div class="menu">
      {#each options as option}
        <button
          type="button"
          class="option"
          class:selected={option.value === value}
          on:click={() => select(option)}
        >
          {option.label}
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .dropdown {
    position: relative;
    display: inline-block;
  }

  .trigger {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.25rem 0.5rem;
    border: 1px solid var(--border-default);
    border-radius: 4px;
    background-color: var(--bg-secondary);
    color: var(--text-primary);
    font-size: 0.8rem;
    cursor: pointer;
    min-width: 100px;
  }

  .trigger:hover {
    border-color: var(--border-focus);
  }

  .arrow {
    font-size: 0.6rem;
    color: var(--text-dim);
  }

  .menu {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 2px;
    min-width: 100%;
    background-color: var(--bg-secondary);
    border: 1px solid var(--border-default);
    border-radius: 4px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
    z-index: 100;
    overflow: hidden;
  }

  .option {
    display: block;
    width: 100%;
    padding: 0.5rem 0.75rem;
    border: none;
    background: none;
    color: var(--text-primary);
    font-size: 0.85rem;
    text-align: left;
    cursor: pointer;
  }

  .option:hover {
    background-color: var(--bg-elevated);
  }

  .option.selected {
    background-color: var(--accent-primary);
    color: var(--color-ghost-white, #F5F5FD);
  }
</style>
