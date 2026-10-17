--[[
kbase-export.lua: the pandoc Lua filter of `kbase export-latex` (pandoc 3.12,
its embedded Lua 5.4), run over the gfm reader's AST of the stream files,
read file-scoped and concatenated in order into one document.

It reads its tables from the JSON file the metadata key `kbase-xref` names.
That path, and the tables' `report` path, resolve against pandoc's working
directory: the export runs pandoc in the defaults file's directory, as the
author's re-run does. Every key is present; an empty table is [] or {}.

  documents     object, heading id -> {path, volume, rank}:
                  path    the document's kb-root-relative slash path
                  volume  N, the 1-based position of its volume in `volumes`
                  rank    0 for a volume index (heading id vol{N}), 1 for the
                          documents its index lists, one deeper per index
  paths         object, kb-root-relative path -> heading id (a documents key)
  volumes       array in volume order of {label = "vol{N}", title}
  theorems      array of the names a labelled blockquote's label line may carry
  bibliography  object, citation key -> entry body as gfm text
  report        where the findings are written: a JSON array of
                {form, path, reason}, path the document's kb-root-relative path
  assets        object, kb-root-relative path of a copied file -> its path
                relative to the out directory, assets/<kb-root-relative path>
  entry_point   the entry point's kb-root-relative path, the front matter's
                source
  streams       array of the stream files' names, in reading order; under
                file-scope pandoc prefixes every Markdown-parsed id and every
                #-link target with "<name>__", which the filter removes first

The document a node belongs to is the last Header whose id is a documents key
at or before it in document order; before the first, the front matter, a
document of volume 0 at path entry_point.
]]

local stringify = pandoc.utils.stringify

local tables
local findings = pandoc.List{}
local labels = {} -- labels[volume][label]: the author labels a volume defines
local citations = pandoc.List{} -- the keys \cite names, in first-emission order
local cited = {}
local theorem_names = {}
local theorems_used = pandoc.List{} -- {env, name} in first-use order
local current
local in_note = false -- pass 2 is inside a Note

local function start_of_stream()
  current = { path = tables.entry_point, volume = 0, rank = 0, level = 1 }
end

local function note(form, reason, ...)
  findings:insert({ form = form, path = current.path, reason = reason:format(...) })
end

-- enter makes h's document current where h opens one, and returns it.
local function enter(h)
  local d = tables.documents[h.identifier]
  if d then
    current = { path = d.path, volume = d.volume, rank = d.rank, level = h.level }
  end
  return d
end

-- namespaced is the label the export writes for an author label of a volume,
-- spelled so the LaTeX writer prints an id unchanged: it keeps ASCII letters,
-- digits and +-.:;=_, and rewrites any other character to its own ux{hex}
-- spelling, so a raw \label or \ref and an id the writer prints agree.
local function namespaced(volume, label)
  local out = {}
  for _, code in utf8.codes(label) do
    local c = utf8.char(code)
    out[#out + 1] = code < 128 and c:match('^[%w+%-.:;=_]$') or ('ux%x'):format(code)
  end
  return ('v%d-%s'):format(volume, table.concat(out))
end

local spellings = {} -- spellings[namespaced label]: the author label it spells

local function define(label)
  local defined = labels[current.volume] or {}
  labels[current.volume] = defined
  if defined[label] then
    note('label', 'label %s is defined more than once in volume %d', label, current.volume)
  end
  defined[label] = true
  local spelled = namespaced(current.volume, label)
  if spellings[spelled] and spellings[spelled] ~= label then
    note('label', 'labels %q and %q of volume %d are both written %s in LaTeX', spellings[spelled], label,
      current.volume, spelled)
  end
  spellings[spelled] = label
end

local function undefined(volume, names)
  local missing = {}
  for _, label in ipairs(names) do
    if not (labels[volume] and labels[volume][label]) then
      missing[#missing + 1] = label
    end
  end
  return missing
end

local function namespaced_list(volume, names)
  local out = {}
  for i, label in ipairs(names) do
    out[i] = namespaced(volume, label)
  end
  return table.concat(out, ',')
end

-- split is a cleveref argument's labels; the whole argument where it holds none.
local function split(list)
  local out = {}
  for part in list:gmatch('[^,]+') do
    local label = part:match('^%s*(.-)%s*$')
    if label ~= '' then
      out[#out + 1] = label
    end
  end
  return #out > 0 and out or { list }
end

local entities = { amp = '&', lt = '<', gt = '>', quot = '"', apos = "'" }

-- unescape HTML-decodes an attribute value: the export carries it out of the
-- page into LaTeX, where the page's escaping means nothing.
local function unescape(s)
  return (s:gsub('&(#?)([xX]?)(%w+);', function(hash, x, name)
    if hash == '' then
      return entities[x .. name]
    end
    local n = tonumber(name, x == '' and 10 or 16)
    return n and utf8.char(n)
  end))
end

local function attributes(text)
  local out = {}
  for name, value in text:gmatch('([%w%-]+)="([^"]*)"') do
    out[name] = unescape(value)
  end
  return out
end

local function is_raw_html(node)
  return (node.t == 'RawBlock' or node.t == 'RawInline') and node.format == 'html'
end

local function count(s, pattern)
  local n = 0
  for _ in s:gmatch(pattern) do
    n = n + 1
  end
  return n
end

-- The raw html blocks a built KB carries that the html reader makes native,
-- by tag. Its MathML sits inside them.
local html_elements = { figure = 'Figure', table = 'Table' }

-- Spans stay raw open/close pairs, as the gfm reader hands them, so the
-- contract forms inside a figure or table read as they do in prose; tags the
-- reader has no element for (embed) stay raw.
local html_reader = 'html+raw_html-native_spans'

-- placeholder is the html a Markdown block in a raw html run goes in as: its
-- raw html inlines as they stand, every other node a slot element the html
-- reader keeps, to be swapped back for the node itself.
local function placeholder(block, slots)
  if block.t ~= 'Para' and block.t ~= 'Plain' then
    slots[#slots + 1] = block
    return ('<div data-kbase-slot="%d"></div>'):format(#slots)
  end
  local parts = {}
  for _, inline in ipairs(block.content) do
    if is_raw_html(inline) then
      parts[#parts + 1] = inline.text
    else
      slots[#slots + 1] = inline
      parts[#parts + 1] = ('<code data-kbase-slot="%d">.</code>'):format(#slots)
    end
  end
  return table.concat(parts)
end

local function slot(element, slots)
  local k = tonumber(element.attributes['kbase-slot'] or '')
  return k and slots[k]
end

-- embed_as_image is a raw <embed>'s src as an Image, and its close tag as
-- nothing: the html writer writes a PDF image as an embed, and the html
-- reader reads one back as a raw open/close pair. Other raw html is nil.
local function embed_as_image(raw)
  if not is_raw_html(raw) then
    return nil
  elseif raw.text:match('^<embed[%s>/]') then
    return pandoc.Image({}, attributes(raw.text).src or '')
  elseif raw.text == '</embed>' then
    return {}
  end
end

local function file_name(src)
  return src:match('[^/]*$')
end

-- raw_image_source is the src of a raw html node that is one <img> or <embed>
-- tag and nothing else; nil for any other node.
local function raw_image_source(node)
  if not is_raw_html(node) then
    return nil
  end
  local tag, rest = node.text:match('^%s*<(%a+)([^>]*)>%s*$')
  if (tag == 'img' or tag == 'embed') and (rest == '' or rest:match('^[%s/]')) then
    return attributes(rest).src or ''
  end
end

-- raw_images makes each raw <img> or <embed> directly in a block list, or in
-- one of its paragraphs, an Image; it drops one where the list holds a
-- Markdown image of the same file name, the copy a built KB writes beside the
-- tag, whose src keeps the LaTeX source's path.
local function raw_images(blocks)
  local markdown = {}
  blocks:walk({
    Image = function(image)
      markdown[file_name(image.src)] = true
    end,
  })
  -- replaced is node as its image: itself where it is no raw image tag,
  -- nothing where its file is the Markdown image's.
  local function replaced(node)
    local src = raw_image_source(node)
    if not src then
      return { node }
    end
    return markdown[file_name(src)] and {} or { pandoc.Image({}, src) }
  end
  local out = pandoc.Blocks{}
  for _, block in ipairs(blocks) do
    if raw_image_source(block) then
      local image = replaced(block)
      if #image > 0 then
        out:insert(pandoc.Plain(image))
      end
    else
      if block.t == 'Para' or block.t == 'Plain' then
        local inlines = pandoc.Inlines{}
        for _, inline in ipairs(block.content) do
          inlines:extend(replaced(inline))
        end
        block.content = inlines
      end
      out:insert(block)
    end
  end
  return out
end

-- native_html makes each raw html figure or table in a block list native. The
-- gfm reader splits such an element at its blank lines into raw blocks with
-- Markdown between; the run is reassembled up to the element's close and
-- read as html, which also reads its MathML as Math carrying the annotation's
-- TeX. A run that does not read as its one element stays as it is, for
-- residue to report. Raw images outside them go through raw_images.
local function native_html(blocks)
  local out, i = pandoc.Blocks{}, 1
  while i <= #blocks do
    local tag = is_raw_html(blocks[i]) and blocks[i].text:match('^%s*<(%a+)[%s>]')
    local element = html_elements[tag]
    local html, slots, depth, j = {}, {}, 0, i
    while element and j <= #blocks and (j == i or depth > 0) do
      local text = is_raw_html(blocks[j]) and blocks[j].text or placeholder(blocks[j], slots)
      html[#html + 1] = text
      depth = depth + count(text, '<' .. tag .. '[%s>]') - count(text, '</' .. tag .. '>')
      j = j + 1
    end
    local read = element and depth <= 0 and pandoc.read(table.concat(html, '\n'), html_reader).blocks
    if read and #read == 1 and read[1].t == element then
      out:extend(read:walk({
        Code = function(c)
          return slot(c, slots)
        end,
        Div = function(d)
          return slot(d, slots)
        end,
        RawInline = embed_as_image,
        RawBlock = function(raw)
          local image = embed_as_image(raw)
          return image and image.t == 'Image' and pandoc.Plain({ image }) or image
        end,
      }))
      i = j
    else
      out:insert(blocks[i])
      i = i + 1
    end
  end
  return raw_images(out)
end

-- open_tag is the tag name and attributes of a raw html inline opening an a
-- or a span. The reader hands a hard-wrapped tag as one string, newlines in it.
local function open_tag(raw)
  if raw.t ~= 'RawInline' or raw.format ~= 'html' then
    return nil
  end
  local tag, rest = raw.text:match('^%s*<(%a+)(.-)>%s*$')
  if (tag == 'a' or tag == 'span') and (rest == '' or rest:match('^%s')) then
    return tag, attributes(rest)
  end
end

local function is_close(raw, tag)
  return raw.t == 'RawInline' and raw.format == 'html' and raw.text:match('^%s*(.-)%s*$') == '</' .. tag .. '>'
end

local function form_of(tag, attrs)
  if tag == 'a' then
    return (attrs['data-reference'] or '') ~= '' and 'anchor' or nil
  end
  for class in (attrs.class or ''):gmatch('%S+') do
    if class == 'citation' and (attrs['data-cites'] or '') ~= '' then
      return 'citation'
    end
  end
  return (attrs.id or '') ~= '' and 'label' or nil
end

-- element_at reads the contract form opened by inlines[i]: its form, its
-- attributes, the inlines between its open and close tags, and the close
-- tag's index. An element whose close tag never comes is no form.
local function element_at(inlines, i)
  local tag, attrs = open_tag(inlines[i])
  local form = tag and form_of(tag, attrs)
  if not form then
    return nil
  end
  local depth = 0
  for j = i + 1, #inlines do
    if is_close(inlines[j], tag) then
      if depth == 0 then
        return form, attrs, pandoc.Inlines(table.move(inlines, i + 1, j - 1, 1, {})), j
      end
      depth = depth - 1
    elseif open_tag(inlines[j]) == tag then
      depth = depth + 1
    end
  end
end

-- resolve is a link destination as the kb-root-relative path it names from
-- the document at from, the current one by default: fragment and :line suffix
-- dropped, percent-decoded, joined to that document's directory and cleaned.
-- A bare fragment names that document.
local function resolve(target, from)
  from = from or current.path
  target = target:gsub('#.*', ''):gsub(':%d+$', ''):gsub('%%(%x%x)', function(h)
    return string.char(tonumber(h, 16))
  end)
  if target == '' then
    return from
  end
  local segments = {}
  for segment in ((from:match('^(.*)/') or '') .. '/' .. target):gmatch('[^/]+') do
    if segment == '..' and #segments > 0 and segments[#segments] ~= '..' then
      table.remove(segments)
    elseif segment ~= '.' then
      segments[#segments + 1] = segment
    end
  end
  return table.concat(segments, '/')
end

-- exported_document is the documents key of the exported document a link
-- destination names from the current document, the path it names, and for a
-- finding where it names none, what was tried beyond the current document.
-- Inside a Note the destination is also resolved from each exported document
-- of the current volume, and names the one exported document those
-- resolutions name: a built KB relinks a footnote's definition for the
-- document holding it, not the one referencing it.
local function exported_document(target)
  local path = resolve(target)
  if tables.paths[path] or not in_note then
    return tables.paths[path], path, ''
  end
  local seen, named = {}, {}
  for _, d in pairs(tables.documents) do
    local candidate = d.volume == current.volume and resolve(target, d.path)
    if candidate and tables.paths[candidate] and not seen[candidate] then
      seen[candidate] = true
      named[#named + 1] = candidate
    end
  end
  if #named == 1 then
    return tables.paths[named[1]], named[1], ''
  end
  table.sort(named)
  local tried = '; resolved from each document of volume %d, as a footnote may be, it names %s'
  return nil, path, tried:format(current.volume, #named == 0 and 'none' or 'several: ' .. table.concat(named, ', '))
end

local reference_macros = { eqref = '\\eqref', ['ref+label'] = '\\cref', ['ref+Label'] = '\\Cref' }
local math_references = { ref = true, eqref = true, cref = true, Cref = true }

-- The display environments of LaTeX and amsmath that pandoc 3.12's LaTeX
-- writer wraps in \[…\], where they cannot sit: written raw instead.
local raw_display_environments = { ['eqnarray*'] = true, xalignat = true, ['xalignat*'] = true, xxalignat = true }

local rewrite_inlines
local rewrite_document -- pass 2's filter

-- anchor is what a reference anchor becomes. data-reference names the target:
-- an element reference wherever the target volume (the href document's, else
-- the current one) defines every label it names; otherwise, where the href
-- names an exported document, a \ref to that document's heading; otherwise
-- the anchor's own text and a finding.
local function anchor(attrs, content)
  local kind = attrs['data-reference-type'] or ''
  local names = (kind == 'ref+label' or kind == 'ref+Label') and split(attrs['data-reference'])
    or { attrs['data-reference'] }
  local volume, document = current.volume, nil
  local href = (attrs.href or ''):match('^[^#]*')
  if href ~= '' then
    local target, tried
    document, target, tried = exported_document(href)
    if not document then
      note('anchor', 'the anchor names %s, which is not an exported document%s', target, tried)
      return rewrite_inlines(content)
    end
    volume = tables.documents[document].volume
  end
  local missing = undefined(volume, names)
  if #missing == 0 then
    local macro = reference_macros[kind] or '\\ref'
    return { pandoc.RawInline('latex', macro .. '{' .. namespaced_list(volume, names) .. '}') }
  elseif document then
    return { pandoc.RawInline('latex', '\\ref{' .. document .. '}') }
  end
  note('anchor', 'the anchor names %s, which no exported document of volume %d defines',
    table.concat(missing, ', '), volume)
  return rewrite_inlines(content)
end

-- cite is what a citation span becomes: a \cite of its keys where every one
-- has a bibliography entry; otherwise the span's own text and a finding, so
-- no \cite names an undefined key and no work the text names is dropped.
local function cite(keys, content)
  local list, missing = {}, {}
  for key in keys:gmatch('%S+') do
    list[#list + 1] = key
    if not tables.bibliography[key] then
      missing[#missing + 1] = key
    end
  end
  if #missing > 0 then
    note('citation', 'the citation key %s has no bibliography entry; the citation is its text',
      table.concat(missing, ', '))
    return rewrite_inlines(content)
  end
  for _, key in ipairs(list) do
    if not cited[key] then
      cited[key] = true
      citations:insert(key)
    end
  end
  return { pandoc.RawInline('latex', '\\cite{' .. table.concat(list, ',') .. '}') }
end

local residue_attributes = { 'data-reference', 'data-cites', 'id' }

-- residue notes a raw html node no rule claimed that carries a load-bearing
-- attribute: the latex writer drops raw html.
local function residue(raw)
  if raw.format ~= 'html' then
    return
  end
  local tag = raw.text:match('^%s*<(%a+)[%s>]')
  if html_elements[tag] or tag == 'math' or tag == 'img' or tag == 'embed' then
    note(tag, 'raw html <%s> did not read as a native element, and the LaTeX writer drops it: %q', tag, raw.text)
    return
  end
  local attrs, carried = attributes(raw.text), {}
  for _, name in ipairs(residue_attributes) do
    if attrs[name] then
      carried[#carried + 1] = name
    end
  end
  if #carried > 0 then
    note('residue', 'raw html carrying %s was claimed by no rule, and the LaTeX writer drops it: %q',
      table.concat(carried, ', '), raw.text)
  end
end

local function namespace_identifier(element)
  if element.identifier ~= '' then
    element.identifier = namespaced(current.volume, element.identifier)
  end
  return element
end

-- rewrite_inlines replaces each contract form in one inline list, and
-- namespaces a native span's or link's id as a label span's. A label span's
-- content and a degraded anchor's stay in the output, so the walk or this
-- call reaches the forms inside them.
rewrite_inlines = function(inlines)
  local out, i = pandoc.Inlines{}, 1
  while i <= #inlines do
    local form, attrs, content, close = element_at(inlines, i)
    if form == 'anchor' then
      out:extend(anchor(attrs, content))
    elseif form == 'citation' then
      out:extend(cite(attrs['data-cites'], content))
    elseif form == 'label' then
      out:insert(pandoc.Span(content, pandoc.Attr(namespaced(current.volume, attrs.id))))
    else
      local inline = inlines[i]
      if inline.t == 'RawInline' then
        residue(inline)
      elseif inline.t == 'Span' then
        namespace_identifier(inline)
      elseif inline.t == 'Link' and inline.identifier ~= '' then
        -- rewrite_link may replace the link, so its label goes on a span around it.
        local id = inline.identifier
        inline.identifier = ''
        inline = pandoc.Span({ inline }, pandoc.Attr(namespaced(current.volume, id)))
      end
      out:insert(inline)
      close = i
    end
    i = close + 1
  end
  return out
end

local function rewrite_math(m)
  -- A braceless argument only, so \frac{C_{\eqref{x}}} does not swallow the \eqref.
  m.text = m.text:gsub('\\(%a+){([^{}]*)}', function(command, argument)
    if command == 'label' then
      return '\\label{' .. namespaced(current.volume, argument) .. '}'
    elseif not math_references[command] then
      return nil
    end
    local names = (command == 'cref' or command == 'Cref') and split(argument) or { argument }
    local missing = undefined(current.volume, names)
    if #missing > 0 then
      note('math-reference', 'the maths reference \\%s{%s} names %s, which no exported document of volume %d defines',
        command, argument, table.concat(missing, ', '), current.volume)
      return nil
    end
    return '\\' .. command .. '{' .. namespaced_list(current.volume, names) .. '}'
  end)
  -- A blank line ends a paragraph, which maths mode cannot hold.
  m.text = m.text:gsub('\n%s*\n', '\n')
  if m.mathtype == 'DisplayMath' then
    local env = m.text:match('^%s*\\begin{([^}]*)}')
    if raw_display_environments[env] and m.text:match('\\end{([^}]*)}%s*$') == env then
      return pandoc.RawInline('latex', m.text)
    end
  end
  -- The writer closes the maths on the body's last line, so a comment there
  -- swallows the delimiter; its own guard reads \\% as an escaped %.
  local last = m.text:match('[^\n]*$'):gsub('\\\\', '')
  if last:find('^%%') or last:find('[^\\]%%') then
    m.text = m.text .. '\n'
  end
  return m
end

local function is_external(target)
  return target:match('^%a[%w+.%-]*:') or target:match('^//')
end

-- asset is the out-relative path of the copied file a link or image source
-- names; nil where it names none.
local function asset(target)
  return not is_external(target) and tables.assets[resolve(target)] or nil
end

-- rewrite_link makes a reference anchor the html reader read (inside a figure
-- or table) its reference; points a link to an exported document at that
-- document's heading, which the writer prints as \hyperref, and a link to a
-- copied file at the copy; an external link passes; any other becomes its
-- text and a finding. A link around a reference anchor (a LaTeX source's
-- \hyperref[L]{… \ref{L}}) is its content: the anchor's href names the
-- target, where the link's bare fragment would name the current document.
local function rewrite_link(link)
  local reference = link.attributes['reference']
  if reference then
    return anchor({ ['data-reference-type'] = link.attributes['reference-type'], ['data-reference'] = reference,
      href = link.target }, link.content)
  end
  for i = 1, #link.content do
    if element_at(link.content, i) == 'anchor' then
      return link.content:walk(rewrite_document), false
    end
  end
  if is_external(link.target) then
    return nil
  end
  local document, target, tried = exported_document(link.target)
  if document then
    link.target = '#' .. document
    return link
  end
  local copy = tables.assets[target]
  if copy then
    link.target = copy
    return link
  end
  note('link', 'the link names %s, which is not an exported document%s', target, tried)
  -- A topdown walk applies no filter to the inlines a filter returns, but
  -- descends into their children: walked here, they are returned with false.
  return link.content:walk(rewrite_document), false
end

-- rewrite_image points an image at its copy under assets/; one naming no
-- copied file becomes its alt text and a finding. An external image passes.
local function rewrite_image(image)
  if is_external(image.src) then
    return nil
  end
  local copy = asset(image.src)
  if copy then
    image.src = copy
    return image
  end
  note('image', 'the image names %s, which is not an exported asset', resolve(image.src))
  return image.caption:walk(rewrite_document), false
end

-- holds_table: the LaTeX writer writes a figure holding a table without its
-- float, caption or label.
local function holds_table(figure)
  local found = false
  figure:walk({
    Table = function()
      found = true
    end,
  })
  return found
end

local function only_space(block)
  return not block.content:find_if(function(inline)
    return inline.t ~= 'Space' and inline.t ~= 'SoftBreak'
  end)
end

local function is_space(inline)
  return inline.t == 'Space' or inline.t == 'SoftBreak'
end

-- table_caption reads the paragraph pandoc's gfm writer writes after a table
-- with a caption and an id: the caption, then the id as an attribute span of
-- nothing but space. It returns the id and the caption's inlines; nil for any
-- other block.
local function table_caption(block)
  if not block or block.t ~= 'Para' then
    return nil
  end
  local c, n = block.content, #block.content
  while n > 0 and is_space(c[n]) do
    n = n - 1
  end
  local span = c[n]
  if not (span and span.t == 'Span' and span.identifier ~= '' and only_space(span)) then
    return nil
  end
  repeat
    n = n - 1
  until n == 0 or not is_space(c[n])
  if n > 0 then
    return span.identifier, pandoc.Inlines(table.move(c, 1, n, 1, {}))
  end
end

-- captioned_tables gives a table without caption or id the caption paragraph
-- after it, its id on the table: a label left on the paragraph's span would
-- name the enclosing section's counter, not the table's.
local function captioned_tables(blocks)
  local out, i = pandoc.Blocks{}, 1
  while i <= #blocks do
    local block = blocks[i]
    if block.t == 'Table' and block.identifier == '' and #block.caption.long == 0 then
      local id, caption = table_caption(blocks[i + 1])
      if id then
        block.identifier = id
        block.caption = { long = { pandoc.Plain(caption) } }
        i = i + 1
      end
    end
    out:insert(block)
    i = i + 1
  end
  return out
end

-- The LaTeX writer sets a figure's n blocks side by side, 0.9/n of the line
-- each, and prints a width under 0.1 as 9.0e-2, which LaTeX cannot read.
local side_by_side_limit = 9

-- rewrite_figure namespaces a figure's id and drops the image a built KB
-- writes beside each copied one, under the path the LaTeX source gave it: an
-- image naming no copied file in a figure with a copied one of its file name.
-- Past side_by_side_limit blocks, they go one under another. A figure
-- holding a table becomes its body and caption, unfloated, and a finding;
-- pass 1 left its id undefined, so references to it degrade.
local function rewrite_figure(figure)
  if holds_table(figure) then
    note('figure', 'the figure %q holds a table, which LaTeX does not float in a figure; its body and caption are written unfloated',
      figure.identifier)
    local blocks = figure.content:clone()
    blocks:extend(figure.caption.long)
    -- A topdown walk applies no block filter to the blocks a filter returns.
    return blocks:walk(rewrite_document), false
  end
  local copied = {}
  figure:walk({
    Image = function(image)
      if asset(image.src) then
        copied[file_name(image.src)] = true
      end
    end,
  })
  local function emptied(block)
    return only_space(block) and {} or nil
  end
  figure = namespace_identifier(figure:walk({
    Image = function(image)
      if not is_external(image.src) and not asset(image.src) and copied[file_name(image.src)] then
        return {}
      end
    end,
    Plain = emptied,
    Para = emptied,
  }))
  if #figure.content > side_by_side_limit then
    figure.content = { pandoc.Div(figure.content) }
  end
  return figure
end

-- The template's line (letter paper, 1in margins, 10pt) in characters of text,
-- about 4.5pt each, kept short of the 104 that fit; and the gap between two
-- columns. A table whose columns' longest cells and gaps sum past the line
-- overruns it.
local line_characters = 96
local column_gap = 3
-- A word set bold runs up to two fifths wider than those characters, and a
-- word alone in its cell cannot break.
local word_slack = 1.4

-- cell_text is a cell's text as printed, roughly: a formula's TeX less its
-- markup, each control word one character.
local function cell_text(blocks)
  return stringify(blocks:walk({
    Math = function(m)
      return pandoc.Str((m.text:gsub('\\%a+', 'x'):gsub('\\.', 'x'):gsub('[%s{}^_]', '')))
    end,
  }))
end

-- wrap_wide_table gives a table too wide for the line, its column widths all
-- the writer's default, relative widths, so its cells wrap within the line.
-- Columns whose longest cell fits an equal share of what is left keep that
-- width; the rest share the remainder by their longest cells; none is
-- narrower than its longest word with word_slack. A table that fits keeps its
-- natural columns.
local function wrap_wide_table(tbl)
  local n = #tbl.colspecs
  for _, spec in ipairs(tbl.colspecs) do
    if spec[2] then
      return
    end
  end
  local longest, word = {}, {}
  for j = 1, n do
    longest[j], word[j] = 0, 0
  end
  local rows = pandoc.List{}
  rows:extend(tbl.head.rows)
  for _, body in ipairs(tbl.bodies) do
    rows:extend(body.head)
    rows:extend(body.body)
  end
  rows:extend(tbl.foot.rows)
  local spanned = {} -- spanned[j]: rows a rowspan above still covers column j
  for _, row in ipairs(rows) do
    local j = 1
    for _, cell in ipairs(row.cells) do
      while (spanned[j] or 0) > 0 do
        spanned[j] = spanned[j] - 1
        j = j + 1
      end
      if j > n then
        break
      end
      if cell.col_span == 1 then
        local text = cell_text(cell.contents)
        longest[j] = math.max(longest[j], utf8.len(text) or #text)
        -- LaTeX breaks a line after a word's hyphen.
        for w in text:gmatch('[^%s%-]+%-?') do
          word[j] = math.max(word[j], (utf8.len(w) or #w) * word_slack)
        end
      end
      for k = j, math.min(j + cell.col_span - 1, n) do
        spanned[k] = cell.row_span - 1
      end
      j = j + cell.col_span
    end
    for k = j, n do
      spanned[k] = math.max((spanned[k] or 0) - 1, 0)
    end
  end
  local left, total = line_characters - column_gap * (n - 1), 0
  for j = 1, n do
    total = total + longest[j]
  end
  if total <= left then
    return
  end
  local width, open = {}, n
  local function unfixed()
    local sum = 0
    for j = 1, n do
      sum = sum + (width[j] and 0 or longest[j])
    end
    return sum
  end
  repeat
    local share, rest, fixed = left / open, unfixed(), false
    for j = 1, n do
      if not width[j] then
        local w
        if longest[j] <= share then
          w = math.max(longest[j], word[j])
        elseif left * longest[j] / rest < word[j] then
          w = word[j]
        end
        if w then
          width[j], left, open, fixed = w, left - w, open - 1, true
        end
      end
    end
  until not fixed or open == 0
  local rest, sum = unfixed(), 0
  for j = 1, n do
    width[j] = width[j] or left * longest[j] / rest
    sum = sum + width[j]
  end
  for j = 1, n do
    tbl.colspecs[j] = { tbl.colspecs[j][1], width[j] / sum }
  end
end

local function rewrite_table(tbl)
  wrap_wide_table(tbl)
  return namespace_identifier(tbl)
end

-- rewrite_header sets a heading's level by its document's rank: the
-- document's own heading at rank + 1, an in-leaf heading its offset below
-- that. Past level 6 it is a bold paragraph, keeping a document id as a
-- label. Only a document heading keeps its id.
local function rewrite_header(h)
  local opens = enter(h)
  if not opens then
    h.identifier = ''
  end
  local level = current.rank + 1 + (opens and 0 or math.max(h.level - current.level, 1))
  if level <= 6 then
    h.level = level
    return h
  end
  note('heading', 'the heading %q falls at level %d, past the deepest LaTeX numbers; it is a bold paragraph',
    stringify(h), level)
  -- Not walked after: rewrite_inlines would namespace the document span as an author's.
  local bold = pandoc.Strong(h.content:walk(rewrite_document))
  return pandoc.Para({ opens and pandoc.Span({ bold }, pandoc.Attr(h.identifier)) or bold }), false
end

-- label_line is the display name and span id of a labelled blockquote's
-- first block, a label line: **Name**, optionally inside a label span pair,
-- Name in the theorems table.
local function label_line(block)
  if not block or (block.t ~= 'Para' and block.t ~= 'Plain') then
    return nil
  end
  local c, id = block.content, nil
  if #c == 3 then
    local tag, attrs = open_tag(c[1])
    if tag ~= 'span' or form_of(tag, attrs) ~= 'label' or not is_close(c[3], 'span') then
      return nil
    end
    c, id = { c[2] }, attrs.id
  end
  if #c == 1 and c[1].t == 'Strong' and theorem_names[stringify(c[1])] then
    return stringify(c[1]), id
  end
end

-- printed_opening reads the opening a rendered theorem's paragraph starts
-- with, **Name N**. or **Name N** (Title)., Name perhaps empty. It returns
-- Name, the title's inlines (nil without one) and the index of the first
-- inline after the opening; nil where the paragraph does not open so.
local function printed_opening(c)
  local name = c[1] and c[1].t == 'Strong' and stringify(c[1]):match('^(.-) ?%d[%w.]*$')
  if not (name and name:match('^[%a ]*$')) then
    return nil
  end
  local title, after
  if c[2] and c[2].t == 'Str' and c[2].text == '.' then
    after = 3
  elseif c[2] and c[2].t == 'Space' and c[3] and c[3].t == 'Str' and c[3].text:sub(1, 1) == '(' then
    for j = 3, #c do
      if c[j].t == 'Str' and c[j].text:sub(-2) == ').' then
        title = pandoc.Inlines(table.move(c, 3, j, 1, {})):clone()
        title[1].text = title[1].text:sub(2)
        title[#title].text = title[#title].text:sub(1, -3)
        after = j + 1
        break
      end
    end
  end
  if not after then
    return nil
  end
  while c[after] and (c[after].t == 'Space' or c[after].t == 'SoftBreak') do
    after = after + 1
  end
  return name, title, after
end

-- rewrite_blockquote unwraps a labelled blockquote. Where its first body
-- paragraph has the printed-number opening, it is a numbered environment
-- kb-<label name>, displayed under the opening's name, opening in that
-- paragraph so the body runs in beside the head. Otherwise (a proof, which
-- prints its own head and end mark; a LaTeX environment such as minipage)
-- its body is written as it stands, the label span's id kept as a label.
-- Any other blockquote stays a quote.
local function rewrite_blockquote(bq)
  local label_name, id = label_line(bq.content[1])
  if not label_name then
    return nil
  end
  local blocks = bq.content:clone()
  blocks:remove(1)
  local first = blocks[1]
  local name, title, after
  if first and (first.t == 'Para' or first.t == 'Plain') then
    name, title, after = printed_opening(first.content)
  end
  if not name then
    if id then
      -- The walk below namespaces it.
      local label = pandoc.Span({}, pandoc.Attr(id))
      if first and (first.t == 'Para' or first.t == 'Plain') then
        first.content:insert(1, label)
      else
        blocks:insert(1, pandoc.Plain({ label }))
      end
    end
    -- A topdown walk applies no block filter to the blocks a filter returns.
    return blocks:walk(rewrite_document), false
  end
  local env = 'kb-' .. label_name:lower():gsub(' ', '-')
  if not theorems_used:find_if(function(t) return t.env == env end) then
    -- An opening of a bare number (**13**.) prints no name: the label line's stands in.
    theorems_used:insert({ env = env, name = name ~= '' and name or label_name })
  end
  local label = id and '\\label{' .. namespaced(current.volume, id) .. '}' or ''
  local head = pandoc.Inlines{}
  if title then
    head:insert(pandoc.RawInline('latex', '\\begin{' .. env .. '}[{'))
    head:extend(title)
    head:insert(pandoc.RawInline('latex', '}]' .. label))
  else
    head:insert(pandoc.RawInline('latex', '\\begin{' .. env .. '}' .. label))
  end
  local rest = table.move(first.content, after, #first.content, 1, {})
  if #rest > 0 then
    head:insert(pandoc.SoftBreak())
    head:extend(rest)
  end
  first.content = head
  blocks:insert(pandoc.RawBlock('latex', '\\end{' .. env .. '}'))
  return blocks:walk(rewrite_document), false
end

local function bibliography()
  local items = {}
  for _, key in ipairs(citations) do
    local latex = pandoc.write(pandoc.read(tables.bibliography[key], 'gfm'), 'latex'):match('^%s*(.-)%s*$')
    items[#items + 1] = '\\bibitem{' .. key .. '} ' .. latex
  end
  if #items > 0 then
    return pandoc.RawBlock('latex', ('\\begin{thebibliography}{%d}\n%s\n\\end{thebibliography}')
      :format(#items, table.concat(items, '\n')))
  end
end

-- plural is a display name's English plural, by its last word's ending: a
-- name already ending in a plural s (Examples) is its own plural.
local function plural(name)
  local lower = name:lower()
  if lower:match('is$') then
    return name:sub(1, -3) .. 'es'
  elseif lower:match('[^aeiou]y$') then
    return name:sub(1, -2) .. 'ies'
  elseif lower:match('[su]s$') or lower:match('[xz]$') or lower:match('[cs]h$') then
    return name .. 'es'
  elseif lower:match('s$') then
    return name
  end
  return name .. 's'
end

local function add_theorem_declarations(meta)
  local includes = meta['header-includes']
  if includes == nil then
    includes = pandoc.List{}
  elseif pandoc.utils.type(includes) ~= 'List' then
    includes = pandoc.List{ includes }
  end
  for _, t in ipairs(theorems_used) do
    includes:insert(pandoc.MetaBlocks{
      pandoc.RawBlock('latex', ('\\kbasetheorem{%s}{%s}{%s}'):format(t.env, t.name, plural(t.name))),
    })
  end
  meta['header-includes'] = includes
end

local function read_tables(meta)
  local name = meta['kbase-xref']
  if not name then
    error('kbase-export.lua: the metadata key kbase-xref names no tables file', 0)
  end
  local f, err = io.open(stringify(name))
  if not f then
    error('kbase-export.lua: reading the tables: ' .. err, 0)
  end
  local text = f:read('a')
  f:close()
  return pandoc.json.decode(text, false)
end

-- strip_file_scope is doc with pandoc's file-scope prefix "<stream>__" removed
-- from every identifier and every "#<stream>__…" link target, for <stream>
-- exactly a name in tables.streams.
local function strip_file_scope(doc)
  local streams = {}
  for _, name in ipairs(tables.streams) do
    streams[name] = true
  end
  local function unscoped(s)
    local name, rest = s:match('^(.-)__(.*)$')
    if name and streams[name] then
      return rest
    end
    return s
  end
  local function identifier(el)
    el.identifier = unscoped(el.identifier)
    return el
  end
  return doc:walk({
    Header = identifier,
    Div = identifier,
    CodeBlock = identifier,
    Figure = identifier,
    Table = identifier,
    Span = identifier,
    Code = identifier,
    Image = identifier,
    Link = function(link)
      if link.target:sub(1, 1) == '#' then
        link.target = '#' .. unscoped(link.target:sub(2))
      end
      return identifier(link)
    end,
  })
end

local function write_report()
  local f, err = io.open(tables.report, 'w')
  if not f then
    error('kbase-export.lua: writing the report: ' .. err, 0)
  end
  f:write(pandoc.json.encode(findings))
  f:close()
end

function Pandoc(doc)
  tables = read_tables(doc.meta)
  for _, name in ipairs(tables.theorems) do
    theorem_names[name] = true
  end
  doc = strip_file_scope(doc)

  doc = doc:walk({
    Blocks = function(blocks)
      return captioned_tables(native_html(blocks))
    end,
  })

  -- Pass 1: every author label, under the volume of the document it sits in.
  local function define_identifier(element)
    if element.identifier ~= '' then
      define(element.identifier)
    end
  end
  start_of_stream()
  doc:walk({
    traverse = 'topdown',
    Header = function(h)
      enter(h)
    end,
    Figure = function(figure)
      if not holds_table(figure) then
        define_identifier(figure)
      end
    end,
    Table = define_identifier,
    -- Native elements carrying an author id the writer prints as a label.
    -- A heading's is dropped in pass 2, so it is not one.
    Span = define_identifier,
    Link = define_identifier,
    Div = define_identifier,
    CodeBlock = define_identifier,
    Math = function(m)
      for label in m.text:gmatch('\\label{([^}]*)}') do
        define(label)
      end
    end,
    Inlines = function(inlines)
      for i = 1, #inlines do
        local form, attrs = element_at(inlines, i)
        if form == 'label' then
          define(attrs.id)
        end
      end
    end,
  })

  -- Pass 2: the rewrite, in document order.
  start_of_stream()
  rewrite_document = {
    traverse = 'topdown',
    Header = rewrite_header,
    BlockQuote = rewrite_blockquote,
    Figure = rewrite_figure,
    Table = rewrite_table,
    Div = namespace_identifier,
    CodeBlock = namespace_identifier,
    Inlines = rewrite_inlines,
    Math = rewrite_math,
    Link = rewrite_link,
    Image = rewrite_image,
    RawBlock = residue,
    Note = function(n)
      in_note = true
      n.content = n.content:walk(rewrite_document)
      in_note = false
      return n, false
    end,
  }
  doc = doc:walk(rewrite_document)

  local thebibliography = bibliography()
  if thebibliography then
    doc.blocks:insert(thebibliography)
  end
  if #theorems_used > 0 then
    add_theorem_declarations(doc.meta)
  end
  write_report()
  return doc
end
