#### A provider call's failure names what ended it

`provider-refused` — the endpoint answered
with a status that is not success: the request was carried, read and declined, and the reason
carries that status and the provider's own words beside it. `transport-failure` — the
connection, the name, the certificate or the stream failed, and no answer of the provider's was
had. `truncated-empty` — the provider answered, and the answer reached the completion cap before
any answer text: the request leaves it no room, and it is not sent again. A reason states the one that happened: the first calls for the request to be looked at, the
second for the path to the provider. A `provider-refused` 401 or 403 is the provider refusing the
call's key, and its failure item carries the remedy that stores one ([§9](../../9-configuration/index.md), "A refused key").

