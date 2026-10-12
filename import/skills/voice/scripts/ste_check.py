#!/usr/bin/env python3
"""Count STE-lite violations in markdown.

The voice skill runs this in STE mode (Step 2d). It measures only what a
script can measure without guessing: sentence and paragraph length, banned
marks, contractions, and a short swap list. Voice, tense, topic, and term
rotation stay judgment calls.

Usage: ste_check.py FILE [FILE ...]   (a FILE of - reads stdin)
Exit status: 0 when every file is clean, 1 when any file has a violation,
and 2 when a file cannot be read or no file is given.
"""
import html
import re
import statistics
import sys

STEP_MAX, PROSE_MAX, PARA_MAX = 20, 25, 6

# Our own swap list, not the ASD dictionary, which may not be reproduced.
SWAPS = {
    r"\butili[sz](e|es|ed|ing)\b": "use",
    r"\bprior to\b": "before",
    r"\bin order to\b": "to",
    r"\bcommenc(e|es|ed|ing)\b": "start",
    r"\bobtain(s|ed|ing)?\b": "get",
    r"\bensur(e|es|ed|ing)\b": "make sure",
    r"\bperform(s|ed|ing)?\b": "do, run",
    r"\bleverag(e|es|ed|ing)\b": "use",
    r"\bfacilitat(e|es|ed|ing)\b": "help",
    r"\bsubsequently\b": "then",
    r"\bin the event that\b": "if",
}
# Curly forms are built with chr() so that the source stays ASCII.
APOS = "'" + chr(0x2019)
OPEN_QUOTE, CLOSE_QUOTE, EM_DASH = chr(0x201C), chr(0x201D), chr(0x2014)
ABBREV_DOT = chr(0x2024)
# Possessives ("Karpathy's") are not contractions, so 's counts only after
# the pronouns and adverbs that form one.
CONTRACTION = re.compile(
    rf"\b\w+n[{APOS}]t\b|\b\w+[{APOS}](re|ll|ve|m|d)\b"
    rf"|\b(it|that|there|here|what|who|he|she|let|where)[{APOS}]s\b", re.I)
# A code span, with any run of backticks but not an escaped one, or an HTML code element.
CODE = re.compile(r"(?<![`\\])(`+)(?!`).*?(?<!`)\1(?!`)"
                  r"|<(code|kbd|samp)(?:\s[^>]*)?>.*?</\2\s*>", re.I)
# A straight quote opens and closes only next to a space or a mark, so an inch
# mark or a stray quote does not pair with the next real quote. Quotes written
# as character references count too, because a reader sees quote marks.
QUOTE_REF = r"(?:&quot;|&#0*34;|&#[xX]0*22;)"
OPEN_REF = r"(?:&ldquo;|&#0*8220;|&#[xX]0*201[cC];)"
CLOSE_REF = r"(?:&rdquo;|&#0*8221;|&#[xX]0*201[dD];)"
QUOTED = re.compile(rf'(?<![^\W_])"[^"\n]*"(?![^\W_])'
                    rf'|{OPEN_QUOTE}[^{OPEN_QUOTE}{CLOSE_QUOTE}\n]*{CLOSE_QUOTE}'
                    rf'|{QUOTE_REF}.*?{QUOTE_REF}|{OPEN_REF}.*?{CLOSE_REF}')
FOOTNOTE_REF = re.compile(r"\[\^[^\]\s]+\]")
BREAK = re.compile(r"<br\s*/?>", re.I)
COMMENT = re.compile(r"<!--(?:-?>|.*?-->)")
TITLE = r"""(?:"[^"]*"|'[^']*'|\([^)]*\))"""
TARGET = r"(?:[^()\s]|\((?:[^()\s]|\([^()\s]*\))*\))*"
LINK = re.compile(rf"\[([^\]]*)\]\({TARGET}(?:\s+{TITLE})?\s*\)")
LINK_TITLE = re.compile(rf"\]\({TARGET}\s+{TITLE}\s*\)")
# Only the references that CommonMark decodes, so a long number cannot overflow.
ENTITY = re.compile(r"&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{1,31});")
# The dot after these never ends a sentence: initialisms such as e.g. and U.S.,
# vs., cf., approx., incl., the titles Dr., Mr., Mrs., and Ms., and No., Fig.,
# and Sec. before a number. a.m. and p.m. end a sentence unless a time zone or
# a lowercase word follows. So "in the U.S. Then" stays one sentence.
ABBREV = re.compile(r"\b(?:[A-Za-z]\.){2,}(?<![aApP]\.[mM]\.)"
                    r"|\b[aApP]\.[mM]\.(?=\s+(?:[A-Z]{2,}\b|[a-z0-9]))"
                    r"|\b(?:[Vv]s|[Cc]f|[Aa]pprox|[Ii]ncl|Dr|Mr|Mrs|Ms)\."
                    r"|\b(?:[Nn]o|[Ff]ig|[Ss]ec)\.(?=\s*\d)")
# A sentence ends at . ! or ? plus any closing bold, quote, or bracket, and
# the next one starts with a capital, a digit, code, a quote, or a bracket.
SENTENCE_END = re.compile(
    rf"(?:(?<=[.!?])|(?<=[.!?][*_\"){CLOSE_QUOTE}{APOS}])|(?<=[.!?][*_]{{2}}))"
    rf"\s+(?=[A-Z0-9`*_\"{OPEN_QUOTE}(\[])")
TASK_BOX = re.compile(r"\[[ xX]\]\s+")
LIST_MARK = re.compile(r"(?:[-*+]|(\d{1,9})[.)])(?= |$)")
FOOTNOTE_MARK = re.compile(r"\[\^[^\]\s]+\]:(?= |$)")
HEADING = re.compile(r"#{1,6}(\s|$)")
RULE = re.compile(r"([-*_])(\s*\1){2,}\s*")
SETEXT = re.compile(r"=+|-+")
LINK_DEF = re.compile(rf"\[[^\]^][^\]]*\]:\s*(?:<[^<>]*>|\S+)(\s+{TITLE})?\s*")
FENCE = re.compile(r"(`{3,}|~{3,})(.*)")
RAW_OPEN = re.compile(r"<(pre|script|style|textarea)(\s|>|$)", re.I)
RAW_CLOSE = re.compile(r"</(pre|script|style|textarea)>", re.I)
DELIMITER_ROW = re.compile(r"\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?")
ALERT_TAG = re.compile(r"\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]", re.I)
URL = re.compile(rf"<?(?:(?:https?|ftp)://|\bwww\.)"
                 rf"[^\s<>(){EM_DASH}]*[^\s<>().,;:!?'\"{EM_DASH}]>?")
# As in GFM, a reference that ends a URL is not part of it: "a.example&nbsp;".
URL_REFERENCE = re.compile(r"&#?[A-Za-z0-9]+$")
# A removed URL leaves this mark, so a swap phrase cannot join the words around it.
URL_GAP = " " + chr(0xFFFC) + " "


def decode(text):
    """Turn character references into the characters a reader sees."""
    return ENTITY.sub(lambda m: html.unescape(m.group(0)), text)


def mask(text):
    """Blank out code, comments, breaks, quotes, and footnote marks, keeping every offset.

    Inner characters become x, so no mark or sentence end shows through, but
    a quote that ends a sentence keeps its final . ! or ?.
    """
    def quote(m):
        q = m.group(0)
        inner = q[q.index(";") + 1:q.rindex("&")] if q[0] == "&" else q[1:-1]
        end = inner[-1] if inner and inner[-1] in ".!?" else ""
        first, last = ('"', '"') if q[0] == "&" else (q[0], q[-1])
        return first + "x" * (len(q) - 2 - len(end)) + end + last
    text = CODE.sub(lambda m: "`" + "x" * (len(m.group(0)) - 2) + "`", text)
    for pattern in (COMMENT, BREAK, FOOTNOTE_REF):
        text = pattern.sub(lambda m: " " * len(m.group(0)), text)
    return QUOTED.sub(quote, text)


def plain(text):
    """Collapse code or a quote to one word and a link to its text, and drop what shows no word."""
    text = CODE.sub("CODE", text)
    text = LINK.sub(r"\1", COMMENT.sub(" ", BREAK.sub(" ", text)))
    text = QUOTED.sub("QUOTE", text)
    return FOOTNOTE_REF.sub("", text)


def clean(text):
    """Drop link targets and URLs from masked text, then decode what a reader sees."""
    def url(m):
        tail = URL_REFERENCE.search(m.group(0))
        return URL_GAP + (tail.group(0) if tail and m.string.startswith(";", m.end()) else "")
    text = re.sub(r"\]\([^)]*\)", "", LINK.sub(r"\1", text))
    return decode(URL.sub(url, text))


def hidden_dashes(text):
    """Yield the offset of each em-dash in an inline comment or a link title."""
    masked = CODE.sub(lambda m: "x" * len(m.group(0)), text)
    for pattern in (COMMENT, LINK_TITLE):
        for m in pattern.finditer(masked):
            yield from (i for i in range(m.start(), m.end()) if text[i] == EM_DASH)


def frontmatter(lines):
    """Return the length of a leading YAML block, or 0 when there is none."""
    if not lines or lines[0].strip() != "---":
        return 0
    for end, line in enumerate(lines[1:], 1):
        if line.strip() in ("---", "..."):
            body = [s for s in lines[1:end] if s.strip() and not s.lstrip().startswith("#")]
            return end + 1 if body and re.match(r"[\w.-]+\s*:", body[0]) else 0
    return 0


def cells(row):
    """Split a table row on its unescaped pipes."""
    row = re.sub(r"^\||(?<!\\)\|$", "", row.strip())
    return [c.strip() for c in re.split(r"(?<!\\)\|", row)]


def parse(lines):
    """Split markdown into prose units, headings, extra lines, and problems.

    The block structure follows CommonMark, as GitHub renders it. Blockquotes
    and list items are containers: a line stays in each one whose > marker or
    content column it reaches, a paragraph also takes lazy lines, and a fence
    or a comment ends with its container. A unit is a list item, a paragraph
    outside any item, or a table cell. An item holds every paragraph inside
    it, quotes included, but not its nested items. A blockquote reads as prose
    without its > markers. Code is skipped. Frontmatter, comments, and link
    definitions are extras, which count only em-dashes.
    """
    units, headings, extras, problems = [], [], [], []
    stack = []
    para = leaf = table = None
    title_line = 0
    start = frontmatter(lines)
    if start:
        extras += list(enumerate(lines[1:start - 1], 2))

    def add_text(n, text):
        nonlocal para
        if para is None:
            owner = stack[-1]["owner"] if stack else None
            if owner is None:
                owner = {"kind": "prose", "line": None, "paras": []}
                units.append(owner)
            para = {"segs": [], "unit": owner}
            owner["paras"].append(para["segs"])
        para["segs"].append((n, text))
        if text and para["unit"]["line"] is None:
            para["unit"]["line"] = n

    def starts_block(rest, interrupts):
        ind, s = len(rest) - len(rest.lstrip(" ")), rest.strip(" ")
        if ind >= 4 or not s:
            return False
        m = LIST_MARK.match(s)
        if m and not RULE.fullmatch(s):
            starts_at_one = m.group(1) is None or int(m.group(1)) == 1
            return not interrupts or bool(s[m.end():].strip(" ") and starts_at_one)
        return bool(s.startswith((">", "<!--")) or FENCE.match(s) or HEADING.match(s)
                    or RULE.fullmatch(s) or FOOTNOTE_MARK.match(s) or RAW_OPEN.match(s))

    for n, raw in enumerate(lines[start:], start + 1):
        # Only spaces indent and blank a line in CommonMark: a no-break space is text.
        line = raw.expandtabs(4).rstrip(" \r")
        pos = matched = 0
        for c in stack:
            rest = line[pos:]
            if "width" not in c:
                m = re.match(r" {0,3}>", rest)
                if not m:
                    break
                pos += m.end() + (line[pos + m.end():pos + m.end() + 1] == " ")
            elif not rest.strip(" "):
                # A blank line keeps an item open once the item holds something.
                if c["empty"]:
                    break
            else:
                # An item holds a line indented to its content, counted from its container.
                if len(rest) - len(rest.lstrip(" ")) < c["width"]:
                    break
                pos += c["width"]
                c["empty"] = False
            matched += 1
        rest = line[pos:]
        if leaf and matched >= leaf["depth"]:
            if leaf["kind"] == "fence":
                chars = leaf["chars"]
                if len(rest) - len(rest.lstrip(" ")) < 4 and re.fullmatch(
                        re.escape(chars[0]) + "{%d,}" % len(chars), rest.strip(" ")):
                    leaf = None
            elif leaf["kind"] == "html":
                if RAW_CLOSE.search(rest):
                    leaf = None
            else:
                head, closed, tail = rest.partition("-->")
                extras.append((n, head))
                if closed:
                    leaf = None
                    if tail.strip(" "):
                        add_text(n, tail.strip(" "))
            continue
        para_matched = para is not None and matched == len(stack)
        lazy = para is not None and not para_matched and rest.strip(" ")
        if lazy and not starts_block(rest, False):
            add_text(n, rest.strip(" "))
            continue
        if matched < len(stack):
            # An open paragraph, fence, comment, or table always sits in the innermost container.
            del stack[matched:]
            para = leaf = table = None
        text_only = False
        while True:
            rest = line[pos:]
            ind, s = len(rest) - len(rest.lstrip(" ")), rest.strip(" ")
            if ind >= 4 or not s or text_only:
                break
            parent = stack[-1] if stack else {"owner": None, "step": False}
            if s.startswith(">"):
                pos += ind + 1 + (line[pos + ind + 1:pos + ind + 2] == " ")
                stack.append({"owner": parent["owner"], "step": parent["step"]})
                para = table = None
                if ALERT_TAG.fullmatch(line[pos:].strip(" ")):
                    # A GitHub alert tag starts the quote's paragraph, but it is not prose.
                    add_text(n, "")
                    pos, text_only = len(line), True
                continue
            m = LIST_MARK.match(s) or FOOTNOTE_MARK.match(s)
            if not m or RULE.fullmatch(s):
                break
            # Only a bullet with text or a list that starts at 1 may interrupt a paragraph.
            if para is not None and m.re is LIST_MARK and not starts_block(rest, True):
                break
            after = s[m.end():]
            gap = len(after) - len(after.lstrip(" "))
            if m.re is FOOTNOTE_MARK:
                width, pos = ind + 4, pos + ind + m.end() + gap
            else:
                width = ind + m.end() + (gap if after.strip(" ") and gap <= 4 else 1)
                pos += width
            step = m.re is LIST_MARK and (parent["step"] or m.group(1) is not None)
            unit = {"kind": "step" if step else "prose", "line": n, "item": True, "paras": []}
            units.append(unit)
            empty = not after.strip(" ")
            stack.append({"width": width, "owner": unit, "step": step, "empty": empty})
            para = table = None
            box = TASK_BOX.match(line[pos:].lstrip(" "))
            if box:
                # A task box is inline text, so the rest of the line is the item's first paragraph.
                pos = len(line) - len(line[pos:].lstrip(" ")) + box.end()
                text_only = True
        if text_only:
            if s:
                add_text(n, s)
            continue
        if not s:
            para = table = None
            continue
        if ind >= 4 and para is None:
            continue
        if ind < 4:
            m = FENCE.match(s)
            if m and not (m.group(1)[0] == "`" and "`" in m.group(2)):
                leaf = {"kind": "fence", "chars": m.group(1), "depth": len(stack), "line": n}
                para = table = None
                continue
            if RAW_OPEN.match(s):
                para = table = None
                if not RAW_CLOSE.search(s):
                    leaf = {"kind": "html", "depth": len(stack), "line": n}
                continue
            if s.startswith("<!--"):
                para = table = None
                end = s.find("-->", 2)
                if end < 0:
                    leaf = {"kind": "comment", "depth": len(stack), "line": n}
                    extras.append((n, s))
                    continue
                extras.append((n, s[:end + 3]))
                if s[end + 3:].strip(" "):
                    add_text(n, s[end + 3:].strip(" "))
                continue
            if HEADING.match(s):
                headings.append((n, s))
                para = table = None
                continue
            header = para["segs"][-1] if para is not None else None
            if header and "|" in s and DELIMITER_ROW.fullmatch(s) and len(cells(header[1])) == len(cells(s)):
                # The delimiter row turns the paragraph's last line into a table header, as in GFM.
                para["segs"].pop()
                in_step = bool(stack) and stack[-1]["step"]
                table = {"width": len(cells(s)), "step": in_step}
                units += [{"kind": "cell", "step": in_step, "paras": [[(header[0], c)]]}
                          for c in cells(header[1]) if c]
                para = None
                continue
            if para is not None and SETEXT.fullmatch(s):
                # The paragraph above a setext underline is a heading.
                para["unit"]["paras"].remove(para["segs"])
                headings.append((para["segs"][0][0], " ".join(t for _, t in para["segs"])))
                para = None
                continue
            if RULE.fullmatch(s):
                para = table = None
                continue
            definition = LINK_DEF.fullmatch(s) if para is None and table is None else None
            if definition:
                extras.append((n, s))
                title_line = 0 if definition.group(1) else n + 1
                continue
            elif n == title_line and para is None and re.fullmatch(TITLE, s):
                # A link definition's title may sit on the next line.
                extras.append((n, s))
                continue
        if table is not None:
            # Rows are read up to the header's width: GFM drops any cell past it.
            units += [{"kind": "cell", "step": table["step"], "paras": [[(n, c)]]}
                      for c in cells(s)[:table["width"]] if c]
            continue
        add_text(n, s)
    if leaf:
        what = {"fence": "fence", "html": "HTML block", "comment": "comment"}[leaf["kind"]]
        message = f"{what} never closes, so the lines after it are not checked"
        problems.append((leaf["line"], message))
    return units, headings, extras, problems


def joined(para):
    """Join a paragraph's segments with spaces and return (text, masked, bounds)."""
    text, bounds = "", []
    for n, seg in para:
        bounds.append((n, len(text), len(text) + len(seg)))
        text += seg + " "
    return text, mask(text), bounds


def sentences(text, masked, bounds):
    """Yield (line, words, sentence) for each sentence of a joined unit."""
    protected = ABBREV.sub(lambda m: m.group(0).replace(".", ABBREV_DOT), masked)
    pos = 0
    for part in SENTENCE_END.split(protected):
        begin = pos + len(part) - len(part.lstrip())
        pos += len(part)
        pos += len(protected[pos:]) - len(protected[pos:].lstrip())
        raw = text[begin:begin + len(part.strip())]
        words = len(plain(raw).split())
        if words:
            line = [n for n, a, _ in bounds if a <= begin][-1]
            yield line, words, raw


def marks(n, line, after, issues, counts):
    """Record the marks, contractions, and swap words in one cleaned line."""
    for label, key, hits in (("em-dash", "em-dashes", line.count(EM_DASH)),
                             ("semicolon", "semicolons", line.count(";"))):
        counts[key] += hits
        issues += [(n, label)] * hits
    for m in CONTRACTION.finditer(line):
        counts["contractions"] += 1
        issues.append((n, f"contraction '{m.group(0)}'"))
    # Join the start of the next line, so that a phrase which wraps still matches.
    window = line + (" " + " ".join(after.split()[:3]) if after else "")
    for pattern, swap in SWAPS.items():
        for m in re.finditer(pattern, window, re.I):
            if m.start() < len(line):
                issues.append((n, f"'{m.group(0)}', write '{swap}'"))


def check(name, text):
    """Print a report for one document and return its violation count."""
    units, headings, extras, issues = parse(text.split("\n"))
    lengths, bold = [], 0
    counts = {"em-dashes": 0, "semicolons": 0, "contractions": 0}
    for u in units:
        count = 0
        for para in u["paras"]:
            text_p, masked, bounds = joined(para)
            for line, words, sent in sentences(text_p, masked, bounds):
                lengths.append(words)
                count += 1
                cap = STEP_MAX if u["kind"] == "step" or u.get("step") else PROSE_MAX
                if words > cap:
                    issues.append((line, f"{words} words (max {cap}): {sent[:70]}"))
            for i in hidden_dashes(text_p):
                counts["em-dashes"] += 1
                issues.append(([n for n, a, _ in bounds if a <= i][-1], "em-dash"))
            cleaned = [(n, clean(masked[a:b])) for n, a, b in bounds]
            for i, (n, line) in enumerate(cleaned):
                bold += len(re.findall(r"\*\*[^*]+\*\*", line))
                marks(n, line, cleaned[i + 1][1] if i + 1 < len(cleaned) else "", issues, counts)
        if u["kind"] != "cell" and count > PARA_MAX:
            issues.append((u["line"], f"paragraph has {count} sentences (max {PARA_MAX})"))
    for n, line in headings:
        marks(n, clean(mask(line)), "", issues, counts)
    for n, line in extras:
        hits = line.count(EM_DASH)
        counts["em-dashes"] += hits
        issues += [(n, "em-dash")] * hits
    med = statistics.median(lengths) if lengths else 0
    print(f"{name}: {len(lengths)} sentences, median {med:g} words, max {max(lengths, default=0)}")
    print("  " + ", ".join(f"{k} {v}" for k, v in counts.items()) + f", bold spans {bold}")
    for line, message in sorted(issues, key=lambda issue: issue[0]):
        print(f"  L{line}: {message}")
    return len(issues)


def main(paths):
    if not paths:
        print(__doc__, file=sys.stderr)
        return 2
    found, unreadable = 0, False
    for path in paths:
        try:
            if path == "-":
                if sys.stdin is None:
                    raise OSError("stdin is closed")
                sys.stdin.reconfigure(encoding="utf-8-sig")
                name, text = "stdin", sys.stdin.read()
            else:
                with open(path, encoding="utf-8-sig") as f:
                    name, text = path, f.read()
        except (OSError, UnicodeDecodeError) as err:
            print(f"{path}: cannot read: {err}", file=sys.stderr)
            unreadable = True
            continue
        found += check(name, text)
    return 2 if unreadable else 1 if found else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
