package blackfriday

import (
	"testing"
)

func doTestsSanitize(t *testing.T, tests []string) {
	doTestsInlineParam(t, tests, 0, HTML_SKIP_STYLE|HTML_SANITIZE_OUTPUT, HtmlRendererParameters{})
}

func TestSanitizeRawHtmlTag(t *testing.T) {
	tests := []string{
		"zz <style>p {}</style>\n",
		"<p>zz &lt;style&gt;p {}&lt;/style&gt;</p>\n",

		"zz <STYLE>p {}</STYLE>\n",
		"<p>zz &lt;style&gt;p {}&lt;/style&gt;</p>\n",

		"<SCRIPT>alert()</SCRIPT>\n",
		"<p>&lt;script&gt;alert()&lt;/script&gt;</p>\n",

		"zz <SCRIPT>alert()</SCRIPT>\n",
		"<p>zz &lt;script&gt;alert()&lt;/script&gt;</p>\n",

		"zz <script>alert()</script>\n",
		"<p>zz &lt;script&gt;alert()&lt;/script&gt;</p>\n",

		" <script>alert()</script>\n",
		"<p>&lt;script&gt;alert()&lt;/script&gt;</p>\n",

		"<script>alert()</script>\n",
		"&lt;script&gt;alert()&lt;/script&gt;\n",

		"<script src='foo'></script>\n",
		"&lt;script src=&#39;foo&#39;&gt;&lt;/script&gt;\n",

		"<script src='a>b'></script>\n",
		"&lt;script src=&#39;a&gt;b&#39;&gt;&lt;/script&gt;\n",

		"zz <script src='foo'></script>\n",
		"<p>zz &lt;script src=&#39;foo&#39;&gt;&lt;/script&gt;</p>\n",

		"zz <script src=foo></script>\n",
		"<p>zz &lt;script src=foo&gt;&lt;/script&gt;</p>\n",

		`<script><script src="http://example.com/exploit.js"></SCRIPT></script>`,
		"&lt;script&gt;&lt;script src=&#34;http://example.com/exploit.js&#34;&gt;&lt;/script&gt;&lt;/script&gt;\n",

		`'';!--"<XSS>=&{()}`,
		"<p>&#39;&#39;;!--&#34;&lt;xss&gt;=&amp;{()}</p>\n",

		"<SCRIPT SRC=http://ha.ckers.org/xss.js></SCRIPT>",
		"<p>&lt;script SRC=http://ha.ckers.org/xss.js&gt;&lt;/script&gt;</p>\n",

		"<SCRIPT \nSRC=http://ha.ckers.org/xss.js></SCRIPT>",
		"<p>&lt;script \nSRC=http://ha.ckers.org/xss.js&gt;&lt;/script&gt;</p>\n",

		`<IMG SRC="javascript:alert('XSS');">`,
		"<p><img></p>\n",

		"<IMG SRC=javascript:alert('XSS')>",
		"<p><img></p>\n",

		"<IMG SRC=JaVaScRiPt:alert('XSS')>",
		"<p><img></p>\n",

		"<IMG SRC=`javascript:alert(\"RSnake says, 'XSS'\")`>",
		"<p><img></p>\n",

		`<a onmouseover="alert(document.cookie)">xss link</a>`,
		"<p><a>xss link</a></p>\n",

		"<a onmouseover=alert(document.cookie)>xss link</a>",
		"<p><a>xss link</a></p>\n",

		`<IMG """><SCRIPT>alert("XSS")</SCRIPT>">`,
		"<p><img>&lt;script&gt;alert(&#34;XSS&#34;)&lt;/script&gt;&#34;&gt;</p>\n",

		"<IMG SRC=javascript:alert(String.fromCharCode(88,83,83))>",
		"<p><img></p>\n",

		`<IMG SRC=# onmouseover="alert('xxs')">`,
		"<p><img src=\"#\"></p>\n",

		`<IMG SRC= onmouseover="alert('xxs')">`,
		"<p><img></p>\n",

		`<IMG onmouseover="alert('xxs')">`,
		"<p><img></p>\n",

		"<IMG SRC=&#106;&#97;&#118;&#97;&#115;&#99;&#114;&#105;&#112;&#116;&#58;&#97;&#108;&#101;&#114;&#116;&#40;&#39;&#88;&#83;&#83;&#39;&#41;>",
		"<p><img></p>\n",

		"<IMG SRC=&#0000106&#0000097&#0000118&#0000097&#0000115&#0000099&#0000114&#0000105&#0000112&#0000116&#0000058&#0000097&#0000108&#0000101&#0000114&#0000116&#0000040&#0000039&#0000088&#0000083&#0000083&#0000039&#0000041>",
		"<p><img></p>\n",

		"<IMG SRC=&#x6A&#x61&#x76&#x61&#x73&#x63&#x72&#x69&#x70&#x74&#x3A&#x61&#x6C&#x65&#x72&#x74&#x28&#x27&#x58&#x53&#x53&#x27&#x29>",
		"<p><img></p>\n",

		`<IMG SRC="javascriptascript:alert('XSS');">`,
		"<p><img></p>\n",

		`<IMG SRC="jav&#x09;ascript:alert('XSS');">`,
		"<p><img></p>\n",

		`<IMG SRC="jav&#x0A;ascript:alert('XSS');">`,
		"<p><img></p>\n",

		`<IMG SRC="jav&#x0D;ascript:alert('XSS');">`,
		"<p><img></p>\n",

		`<IMG SRC=" &#14;  javascript:alert('XSS');">`,
		"<p><img></p>\n",

		`<SCRIPT/XSS SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
		"<p>&lt;script/XSS SRC=&#34;http://ha.ckers.org/xss.js&#34;&gt;&lt;/script&gt;</p>\n",

		"<BODY onload!#$%&()*~+-_.,:;?@[/|\\]^`=alert(\"XSS\")>",
		"<p>&lt;body onload!#$%&amp;()*~+-_.,:;?@[/|\\]^`=alert(&#34;XSS&#34;)&gt;</p>\n",

		`<SCRIPT/SRC="http://ha.ckers.org/xss.js"></SCRIPT>`,
		"<p>&lt;script/SRC=&#34;http://ha.ckers.org/xss.js&#34;&gt;&lt;/script&gt;</p>\n",

		`<<SCRIPT>alert("XSS");//<</SCRIPT>`,
		"<p>&lt;&lt;script&gt;alert(&#34;XSS&#34;);//&lt;&lt;/script&gt;</p>\n",

		"<SCRIPT SRC=http://ha.ckers.org/xss.js?< B >",
		"<p>&lt;script SRC=http://ha.ckers.org/xss.js?&lt; B &gt;</p>\n",

		"<SCRIPT SRC=//ha.ckers.org/.j>",
		"<p>&lt;script SRC=//ha.ckers.org/.j&gt;</p>\n",

		`<IMG SRC="javascript:alert('XSS')"`,
		"<p>&lt;IMG SRC=&#34;javascript:alert(&#39;XSS&#39;)&#34;</p>\n",

		"<iframe src=http://ha.ckers.org/scriptlet.html <",
		// The hyperlink gets linkified, the <iframe> gets escaped
		"<p>&lt;iframe src=<a href=\"http://ha.ckers.org/scriptlet.html\">http://ha.ckers.org/scriptlet.html</a> &lt;</p>\n",

		// Additonal token types: SelfClosing, Comment, DocType.
		"<br/>",
		"<p><br/></p>\n",

		"<!-- Comment -->",
		"<!-- Comment -->\n",

		"<!DOCTYPE test>",
		"<p>&lt;!DOCTYPE test&gt;</p>\n",
	}
	doTestsSanitize(t, tests)
}

func TestSanitizeQuoteEscaping(t *testing.T) {
	tests := []string{
		// Make sure quotes are transported correctly (different entities or
		// unicode, but correct semantics)
		"<p>Here are some &quot;quotes&quot;.</p>\n",
		"<p>Here are some &#34;quotes&#34;.</p>\n",

		"<p>Here are some &ldquo;quotes&rdquo;.</p>\n",
		"<p>Here are some \u201Cquotes\u201D.</p>\n",

		// Within a <script> tag, content gets parsed by the raw text parsing rules.
		// This test makes sure we correctly disable those parsing rules and do not
		// escape e.g. the closing </p>.
		`Here are <script> some "quotes".`,
		"<p>Here are &lt;script&gt; some &#34;quotes&#34;.</p>\n",

		// Same test for an unknown element that does not switch into raw mode.
		`Here are <eviltag> some "quotes".`,
		"<p>Here are &lt;eviltag&gt; some &#34;quotes&#34;.</p>\n",
	}
	doTestsSanitize(t, tests)
}

func TestSanitizeSelfClosingTag(t *testing.T) {
	tests := []string{
		"<hr>\n",
		"<hr>\n",

		"<hr/>\n",
		"<hr/>\n",

		// Make sure that evil attributes are stripped for self closing tags.
		"<hr onclick=\"evil()\"/>\n",
		"<hr/>\n",
	}
	doTestsSanitize(t, tests)
}

func TestSanitizeInlineLink(t *testing.T) {
	tests := []string{
		"[link](javascript:evil)",
		"<p><a>link</a></p>\n",
		"[link](/abc)",
		"<p><a href=\"/abc\">link</a></p>\n",
	}
	doTestsSanitize(t, tests)
}
