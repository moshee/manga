This is the steaming turd I wrote over the years to automate scanlation tasks from scan to release. The code can all be considered in the public domain. It will not be maintained.

It's a subcommand-based CLI tool with a number of tasks available as subcommands. Invoke with no arguments for usage info. Or just read the code.

Some things are broken, most notably the Batoto uploader which has some concurrency issues causing it to return early sometimes before all of the files are uploaded. At worse it can be used as a reference to implement your own custom Batoto batch uploader.
