### Serialization formats

JSONL for flat records, YAML for documents, TOML for flat configuration. One named departure (SPEC
§10): the KB's two record files, `.index/nodes.yaml` and `.index/edges.yaml`, are each one YAML
document, a block list of block mappings, so the whole metadata layer has one serialisation. The
trap: a record spans several lines, so a line-oriented tool — `grep`, a line count, a line-wise
merge — sees fields, never whole records; read them through the record loader
([reads through `kb.Source`](reads-through-kb-source.md)).
