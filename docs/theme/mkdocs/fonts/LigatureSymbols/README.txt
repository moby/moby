
This font was found and taken from:
http://kudakurage.com/ligature_symbols/


Use it as follows:

<!-- HTML -->

<p>Simple use for mailto link.</p>
<a href="mailto:mail@example.com" class="lsf">mail</a>

<p>Use tha icon with text.</p>
<a href="http://twitter.com/" class="lsf-icon" title="twitter">Twitter</a>

<p>Use tha icon with unicode.</p>
<a href="http://amazon.com/" class="lsf-icon amazon">Amazon</a>


/* CSS */

@font-face {
    font-family: 'LigatureSymbols';
    src: url('LigatureSymbols-2.11.eot');
    src: url('LigatureSymbols-2.11.eot?#iefix') format('embedded-opentype'),
         url('LigatureSymbols-2.11.woff') format('woff'),
         url('LigatureSymbols-2.11.ttf') format('truetype'),
         url('LigatureSymbols-2.11.svg#LigatureSymbols') format('svg');
    src: url('LigatureSymbols-2.11.ttf') format('truetype');
    font-weight: normal;
    font-style: normal;
}

.lsf, .lsf-icon:before {
  font-family: 'LigatureSymbols';
  -webkit-text-rendering: optimizeLegibility;
  -moz-text-rendering: optimizeLegibility;
  -ms-text-rendering: optimizeLegibility;
  -o-text-rendering: optimizeLegibility;
  text-rendering: optimizeLegibility;
  -webkit-font-smoothing: antialiased;
  -moz-font-smoothing: antialiased;
  -ms-font-smoothing: antialiased;
  -o-font-smoothing: antialiased;
  font-smoothing: antialiased;
}

.lsf-icon:before {
  content:attr(title);
  margin-right:0.3em;
  font-size:130%;
}

.lsf-icon.amazon:before {
  content: '\E007';
}





Use by including this in your style sheet:



// LigatureSymbols font kit
// ----------------------------------- // -----------------------------------


@font-face {
    font-family: 'LigatureSymbols';
    src: url('../fonts/LigatureSymbols/LigatureSymbols-2.11.eot');
    src: url('../fonts/LigatureSymbols/LigatureSymbols-2.11.eot?#iefix') format('embedded-opentype'),
         url('../fonts/LigatureSymbols/LigatureSymbols-2.11.woff') format('woff'),
         url('../fonts/LigatureSymbols/LigatureSymbols-2.11.ttf') format('truetype'),
         url('../fonts/LigatureSymbols/LigatureSymbols-2.11.svg#LigatureSymbols') format('svg');
    font-weight: normal;
    font-style: normal;
}

.lsf {
  font-family: 'LigatureSymbols';
  -webkit-text-rendering: optimizeLegibility;
  -moz-text-rendering: optimizeLegibility;
  -ms-text-rendering: optimizeLegibility;
  -o-text-rendering: optimizeLegibility;
  text-rendering: optimizeLegibility;
  -webkit-font-smoothing: antialiased;
  -moz-font-smoothing: antialiased;
  -ms-font-smoothing: antialiased;
  -o-font-smoothing: antialiased;
  font-smoothing: antialiased;
  -webkit-font-feature-settings: "liga" 1, "dlig" 1;
  -moz-font-feature-settings: "liga=1, dlig=1";
  -ms-font-feature-settings: "liga" 1, "dlig" 1;
  -o-font-feature-settings: "liga" 1, "dlig" 1;
  font-feature-settings: "liga" 1, "dlig" 1;
}
.lsf-icon:before {
  content:attr(title);
  margin-right:0.3em;
  font-size:130%;
  font-family: 'LigatureSymbols';
  -webkit-text-rendering: optimizeLegibility;
  -moz-text-rendering: optimizeLegibility;
  -ms-text-rendering: optimizeLegibility;
  -o-text-rendering: optimizeLegibility;
  text-rendering: optimizeLegibility;
  -webkit-font-smoothing: antialiased;
  -moz-font-smoothing: antialiased;
  -ms-font-smoothing: antialiased;
  -o-font-smoothing: antialiased;
  font-smoothing: antialiased;
  -webkit-font-feature-settings: "liga" 1, "dlig" 1;
  -moz-font-feature-settings: "liga=1, dlig=1";
  -ms-font-feature-settings: "liga" 1, "dlig" 1;
  -o-font-feature-settings: "liga" 1, "dlig" 1;
  font-feature-settings: "liga" 1, "dlig" 1;
}





And then practice it by:

