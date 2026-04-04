import { e as escape_html, a0 as attr_class, a1 as attr_style, a2 as ensure_array_like, $ as derived } from "../../chunks/renderer.js";
import "clsx";
function Display($$renderer, $$props) {
  let { previous, current } = $$props;
  $$renderer.push(`<div class="display svelte-18tixtq"><div class="display-previous svelte-18tixtq">${escape_html(previous)}</div> <div class="display-current svelte-18tixtq">${escape_html(current)}</div></div>`);
}
function CalcButton($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { label, type = "default", span = 1 } = $$props;
    let pressed = false;
    $$renderer2.push(`<button${attr_class("btn svelte-18nsvt7", void 0, {
      "operator": type === "operator",
      "equals": type === "equals",
      "clear": type === "clear",
      "zero": span === 2,
      "pressed": pressed
    })}${attr_style(span === 2 ? "grid-column: span 2" : "")}>${escape_html(label)}</button>`);
  });
}
function ButtonGrid($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { onNumber, onOperation, onCalculate, onClear, onDelete } = $$props;
    const buttons = [
      { label: "C", type: "clear", action: () => onClear() },
      { label: "⌫", type: "clear", action: () => onDelete() },
      { label: "%", type: "operator", action: () => onOperation("%") },
      { label: "÷", type: "operator", action: () => onOperation("÷") },
      { label: "7", action: () => onNumber("7") },
      { label: "8", action: () => onNumber("8") },
      { label: "9", action: () => onNumber("9") },
      { label: "×", type: "operator", action: () => onOperation("×") },
      { label: "4", action: () => onNumber("4") },
      { label: "5", action: () => onNumber("5") },
      { label: "6", action: () => onNumber("6") },
      { label: "−", type: "operator", action: () => onOperation("-") },
      { label: "1", action: () => onNumber("1") },
      { label: "2", action: () => onNumber("2") },
      { label: "3", action: () => onNumber("3") },
      { label: "+", type: "operator", action: () => onOperation("+") },
      { label: "0", span: 2, action: () => onNumber("0") },
      { label: ".", action: () => onNumber(".") },
      {
        label: "=",
        type: "equals",
        span: 2,
        action: () => onCalculate()
      }
    ];
    $$renderer2.push(`<div class="buttons svelte-18tcpyk"><!--[-->`);
    const each_array = ensure_array_like(buttons);
    for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
      let btn = each_array[$$index];
      CalcButton($$renderer2, {
        label: btn.label,
        type: btn.type ?? "default",
        span: btn.span ?? 1,
        onpress: btn.action
      });
    }
    $$renderer2.push(`<!--]--></div>`);
  });
}
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    window.matchMedia("(display-mode: standalone)").matches || window.navigator.standalone === true;
    const _saved = JSON.parse(localStorage.getItem("knitnox-calculator") || "null");
    let currentValue = _saved?.currentValue ?? "0";
    let previousValue = _saved?.previousValue ?? "";
    let operation = _saved?.operation ?? null;
    let shouldReset = _saved?.shouldReset ?? false;
    let previousDisplay = derived(() => previousValue && operation ? `${previousValue} ${operation}` : "");
    function appendNumber(num) {
      if (shouldReset) {
        currentValue = "0";
        shouldReset = false;
      }
      if (num === "." && currentValue.includes(".")) return;
      if (currentValue === "0" && num !== ".") {
        currentValue = num;
      } else {
        currentValue += num;
      }
    }
    function setOperation(op) {
      if (operation !== null && previousValue !== "") calculate();
      operation = op;
      previousValue = currentValue;
      shouldReset = true;
    }
    function calculate() {
      if (operation === null || previousValue === "") return;
      const prev = parseFloat(previousValue);
      const cur = parseFloat(currentValue);
      let result;
      switch (operation) {
        case "+":
          result = prev + cur;
          break;
        case "-":
          result = prev - cur;
          break;
        case "×":
          result = prev * cur;
          break;
        case "÷":
          result = cur !== 0 ? prev / cur : "Error";
          break;
        case "%":
          result = prev % cur;
          break;
        default:
          return;
      }
      currentValue = result.toString();
      if (currentValue !== "Error" && currentValue.length > 12) {
        currentValue = parseFloat(currentValue).toExponential(6);
      }
      operation = null;
      previousValue = "";
      shouldReset = true;
    }
    function clearAll() {
      currentValue = "0";
      previousValue = "";
      operation = null;
      shouldReset = false;
    }
    function deleteLast() {
      if (currentValue.length > 1) {
        currentValue = currentValue.slice(0, -1);
      } else {
        currentValue = "0";
      }
    }
    $$renderer2.push(`<div class="page svelte-1uha8ag"><div class="calculator svelte-1uha8ag"><div class="header svelte-1uha8ag"><h1 class="svelte-1uha8ag">Calculator</h1></div> <div class="calc-body svelte-1uha8ag">`);
    Display($$renderer2, { previous: previousDisplay(), current: currentValue });
    $$renderer2.push(`<!----> `);
    ButtonGrid($$renderer2, {
      onNumber: appendNumber,
      onOperation: setOperation,
      onCalculate: calculate,
      onClear: clearAll,
      onDelete: deleteLast
    });
    $$renderer2.push(`<!----></div> <div class="footer svelte-1uha8ag"><a href="/" class="svelte-1uha8ag">← Back to Apps</a></div></div></div> `);
    {
      $$renderer2.push("<!--[-1-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
