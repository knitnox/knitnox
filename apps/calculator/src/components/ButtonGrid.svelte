<script>
  import CalcButton from './CalcButton.svelte';

  let { onNumber, onOperation, onCalculate, onClear, onDelete } = $props();

  const buttons = [
    { label: 'C',   type: 'clear',    action: () => onClear() },
    { label: '⌫',   type: 'clear',    action: () => onDelete() },
    { label: '%',   type: 'operator', action: () => onOperation('%') },
    { label: '÷',   type: 'operator', action: () => onOperation('÷') },
    { label: '7',   action: () => onNumber('7') },
    { label: '8',   action: () => onNumber('8') },
    { label: '9',   action: () => onNumber('9') },
    { label: '×',   type: 'operator', action: () => onOperation('×') },
    { label: '4',   action: () => onNumber('4') },
    { label: '5',   action: () => onNumber('5') },
    { label: '6',   action: () => onNumber('6') },
    { label: '−',   type: 'operator', action: () => onOperation('-') },
    { label: '1',   action: () => onNumber('1') },
    { label: '2',   action: () => onNumber('2') },
    { label: '3',   action: () => onNumber('3') },
    { label: '+',   type: 'operator', action: () => onOperation('+') },
    { label: '0',   span: 2,          action: () => onNumber('0') },
    { label: '.',   action: () => onNumber('.') },
    { label: '=',   type: 'equals',   span: 2, action: () => onCalculate() },
  ];
</script>

<div class="buttons">
  {#each buttons as btn}
    <CalcButton
      label={btn.label}
      type={btn.type ?? 'default'}
      span={btn.span ?? 1}
      onpress={btn.action}
    />
  {/each}
</div>

<style>
  .buttons {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 14px;
  }
</style>
