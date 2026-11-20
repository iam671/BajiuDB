# Empty Access containers

`blank.mdb` and `blank.accdb` are newly generated, empty database containers,
created with a locally installed Microsoft ACE provider and ADOX Catalog.Create.
They contain only the engine's required system objects, with no user tables,
records, credentials, or workstation paths.

The Windows exporter writes a copy to its private staging directory, then uses
the installed ACE provider to create and populate the requested data table.
This avoids mixing ADOX creation and managed OLE DB operations at runtime.
The templates do not contain or redistribute the ACE driver or Access program.
