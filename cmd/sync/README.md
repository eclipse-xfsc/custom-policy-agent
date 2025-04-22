# Sync

Sync is a small Go program used for synchronization between Git repository containing Rego policies
and a MongoDB collection storing policies.

It can also be started as a long-running process which is performing the sync on a given `syncInterval`.

## Functionality

The `sync` program executes the following steps:
* Clones the Rego Git repo on the local filesystem
* Fetches all Repo policy documents from the MongoDB policy collection
* Compares policies from the Git repo and the MongoDB collection
* Inserts new policies and updates modified ones in MongoDB
* Deletes cloned repository from local filesystem (cleanup)

## Build 

The program is written in [Go](https://go.dev/dl/). In order to use it as an executable binary, 
it should be built by running the following command from the root of the repository:
```go
cd cmd/sync

go build -mod=vendor
```
Now an executable binary called `sync` is available in the current directory.

## Usage

Basic usage documentation is available when executing the following command on MacOS or Linux:
```shell
./sync --help
```

The flags passed to the script are as follows:
```    
    -dbAddr string
        Mongo DB connection string.
    -dbUser string
        Mongo DB username.
    -dbPass string
        Mongo DB password.
    -repoURL string
        Policy repository URL.
    -repoUser string
        GIT Server username - optional 
    -repoPass string
        GIT Server password - optional
    -repoFolder string
        Folder where the tool scans for policies - optional
    -branch string
        GIT branch for explicit checkout - optional
    -keepAlive bool
        Keep alive the service (e.g.for containers) - optional
    -syncInterval time.Duration
        Sync interval given as time duration string (e.g. 1s, 10m, 1h30m) - optional
```

Usage example:
```shell
./sync -repoURL="https://path/to/repo.git" -repoUser="user" -repoPass="pass" -dbAddr="mongodb://localhost:27017/policy?directConnection=true" -dbUser="user" -dbPass="pass" -branch="feature-branch" -keepAlive=true -syncInterval=20s
```
