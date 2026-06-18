<script>
  export let section = null
  export let optionsState = {}
  export let configMessage = ''
  export let configMessageType = 'info'

  export let onOptionChange = () => {}
</script>

<div class="settings-pane">
  <h1>{section?.title ?? 'Settings'}</h1>
  {#if configMessage}
    <div class="options-message" data-type={configMessageType}>{configMessage}</div>
  {/if}
  {#if section}
    <div class="options-section">
      {#each section.options as option}
        <div class="option-row">
          <div class="option-text">
            <div class="option-label">{option.label}</div>
            <div class="option-desc">{option.description}</div>
          </div>
          {#if option.type === 'toggle'}
            <button
              type="button"
              class="toggle"
              class:on={optionsState[option.key]}
              aria-label={option.label}
              aria-pressed={optionsState[option.key]}
              on:click={() => onOptionChange(option.key, !optionsState[option.key])}
            ></button>
          {:else if option.type === 'select'}
            <select
              value={optionsState[option.key]}
              on:change={e => onOptionChange(option.key, e.target.value)}
            >
              {#each option.choices as choice}
                <option value={choice}>{choice}</option>
              {/each}
            </select>
          {:else}
            <input
              type="text"
              id="option-{option.key}"
              aria-label={option.label}
              value={optionsState[option.key]}
              on:change={e => onOptionChange(option.key, e.target.value)}
            />
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .settings-pane {
    padding: 1rem 1.25rem;
    overflow-y: auto;
    height: 100%;
  }
  h1 {
    font-size: 1.1rem;
    color: var(--accent-primary);
    margin-bottom: 1rem;
  }
  .options-section {
    margin-bottom: 1.25rem;
  }
  .option-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--border-default);
  }
  .option-label { font-size: 0.85rem; }
  .option-desc { font-size: 0.75rem; color: var(--text-dim); }
  .options-message {
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--border-default);
    margin-bottom: 0.75rem;
    font-size: 0.75rem;
  }
  .options-message[data-type='success'] {
    border-color: rgba(118, 185, 0, 0.4);
    color: var(--success);
  }
  .options-message[data-type='error'] {
    border-color: rgba(255, 107, 107, 0.4);
    color: var(--error);
  }
  .toggle {
    width: 2.25rem;
    height: 1.1rem;
    background: var(--bg-tertiary);
    border: 1px solid var(--border-default);
    cursor: pointer;
  }
  .toggle.on { background: var(--accent-primary); }
</style>
