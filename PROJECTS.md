# Notes

Notes is a backend and frontend system that supports the following:

1. Asynchronously syncs Supernotes uploaded to Google Drive with local
   storage.
2. Automatic conversion of those Supernotes into web visible note pages
3. Connecting a notebook with a github project
4. A web UI to support:
   1. SHowing the notes page
   1. Selecting part of the notes and using this to create a github issue
   1. Marking a notes page as processed

## Techinical Stack

The backend is a golang system, using modern go principles

The whole system is hosted in Kubernetes, using github oauth for auth.