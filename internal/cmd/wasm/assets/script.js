'use strict';

const TextBox = document.querySelector('#text');
TextBox.addEventListener('input', onStateChange);

const OutBox = document.querySelector('#tokens');

let radioText = document.querySelector('#showText');
let radioTokens = document.querySelector('#showTokens');
radioText.addEventListener('change', onStateChange);
radioTokens.addEventListener('change', onStateChange);

function init() {
    // Trigger a redraw to get started.
    onStateChange();
}

// TODO: newlines not working great

//------------------

function onStateChange() {
    const text = TextBox.value;

    if (radioTokens.checked) {
        const start = performance.now();
        let tokens = textToIDs(text);
        const end = performance.now();
        console.log("textToIDs elapsed (ms): ", end - start);
        OutBox.textContent = "[" + tokens.join(", ") + "]";
    } else {
        const start = performance.now();
        let pieces = textToPieces(text);
        const end = performance.now();
        console.log("textToPieces elapsed (ms): ", end - start);
        console.log(pieces);

        OutBox.innerHTML = '';
        // To have different background colors for each piece, we need to
        // wrap each piece in a span. The color is cycled between 8 different
        // colors, in jumps of 135 degrees to make them sufficiently far apart
        // and not repeat for 8 cycles (since 360/8 = 45, we could use any
        // multiple of 45 that's not also a multiple of 180).
        for (let i = 0; i < pieces.length; i++) {
            let color = i % 8;
            let span = document.createElement('span');
            span.textContent = pieces[i];
            span.style.lineHeight = 1.5;
            span.style.backgroundColor = `hsl(${color * 135}, 40%, 70%)`;
            span.style.whiteSpace = 'pre';
            span.style.display = 'inline-block';
            OutBox.appendChild(span);
        }
    }
}
