#### The scoring role

The calls go to the `scoring` role ([§9](../../9-configuration/index.md)): `models.scoring`. The role is the KB's
declaration of which model judges it, so a run that names none refuses as setup — naming that key,
before the KB is opened and before the run's start refresh — and nothing falls back to the text role.
The role's model is the file's `<provider>/<model>` reference or, where the files name none, the one
`--scoring-model` names: the flag supplies an unconfigured role and overrides a configured one, its
reference is resolved against the pool, and a provider the pool does not hold refuses naming the flag
with the pool listed. A run records the model as the flag's where the flag named it. A `--dry-run`
resolves no role and refuses nothing for one.

