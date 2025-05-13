const spawn = (name, props, attrs) => {
  const el = document.createElement(name)
  if (props)
    for (let key in props)
      el[key] = props[key]
  if (attrs)
    for (let key in attrs)
      el.setAttribute(key, attrs[key])
  return el
}

const pre = textContent => spawn('pre', { textContent })

class HTMLImportElement extends HTMLElement {
  static observedAttributes = ['src']

  constructor() {
    super()
  }

  attributeChangedCallback(name, oldValue, newValue) {
    this.innerHTML = ''
    fetch(newValue)
      .then(r => r.ok ? r.text() : newValue + ": " + r.statusText)
      .catch(e => String(e))
      .then(v => {
        if (this.getAttribute("pre") !== null)
          this.appendChild(pre(v))
        else
          this.innerHTML = v
      })
  }
}

customElements.define("html-import", HTMLImportElement)