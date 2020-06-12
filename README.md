# Blue

## Develop
Make sure you add GOPRIVATE to your shell environment:

```
export GOPRIVATE="github.com/ernestrc"
```

If you do not have permission to create a GC credentials file for yourself, ask someone to do that for you.

- Make sure you are in the "Blue Dev" project.
- In the GCP Console, go to APIs & Services. Then go to Credentials.
- Select Create Credentials / Service account.
- Input "your-name-develop" as the service account name and select "Cloud Datastore Owner" in roles and.
- Create service account then on the service account page select "Add Key".
- Select JSON as the Key Type and press "Create".
- Move the downloaded JSON key to a convenient location like inside the config folder.

Next, create a `config/application.yaml` wich points the application to your creds file:

```yaml
gc:
    project-id: "dev"
    creds-file: "/home/ernestrc/src/go/src/github.com/ernestrc/blue/config/gc-creds-2.json"
```
