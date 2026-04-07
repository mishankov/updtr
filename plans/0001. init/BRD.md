I want to create golang cli application called updtr that would help to update dependencies of a project in a variaty of ecosystems.

We will start with Golang ecosystem for now. But architecture shoud support easy extensibility for other ecosystems.

CLI should have an configuration `.toml` file with this configurations options among others:

- Carantine period - dependencies should be old enough befor we update avoid vulnerabilities or malicious updates
