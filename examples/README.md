# Oasis CLI Examples

This folder contains example scenarios for the Oasis CLI. The snippets are
included in the documentation and also serve as a check for potential
regressions in the CLI.

Each example is stored inside its own folder. The folder contains:

- one or more invocation files,
- output file for each invocation file with the content of the resulting
  standard output,
- input and output artifacts
- `config` folder containing custom config file to be used for the scenario.

## Invocation Files

Invocation files have `.in` extension and they will be executed in
alphabetic order. This is important, if you have destructive operations (e.g
adding or removing a wallet) where the order of execution should be respected.
In this case, name the files starting with a number, for example `00-create.in`.

Each `.in` file begins with `oasis` command, which will be replaced with the
path to the actual Oasis CLI command when generating example outputs.

An example invocation file content to create a new wallet:

```
oasis wallet create john
```

### Non-Interactive Execution

If you want to invoke Oasis in a non-interactive mode (by appending
`-y -o /dev/null` parameters), replace the `.in` extension with `.y.in`. For
example `00-create.in` becomes `00-create.y.in`.

### Custom Config Files

Sometimes, you want to use a predefined config file for the Oasis CLI. Put
your desired `cli.toml` and the wallet files to the `config` subfolder
inside your example folder. The folder will be then copied over to a temporary
location before invoking the first file and then fed to CLI by passing the
corresponding `--config` parameter. This way, you can prepare and execute
CLI in an already prepared environment without a dozen of presteps.

### Example Artifacts

If an example requires external files such as a JSON file containing an entity
descriptor or a raw transaction, simply put it alongside the input file.
Assume the working directory will be the one that the input file resides in.
The same goes for the output artifacts (e.g. signed transaction).

## Output Files

The Oasis CLI output for the given input will be stored in a file named the same
as the corresponding invocation file, but having `.out` extension instead of
`.in`.

Scenarios should be designed in a way that the output files remain equal unless
a different behavior of the Oasis CLI is expected.

## Static Examples

If you do not want the example to be executed, but you simply want to store
Oasis CLI execution snippets for example to be included in the documentation,
replace `.in` and `.out` extension with `.in.static` and `.out.static`
respectively. Such files will not be tested and regenerated each time, but you
will have to update it manually. We discourage using this mechanism, but it may
be useful in cases when the output is expected to change and would not make
sense to update it each time (e.g. `oasis network status` returns the current
block height).

## Running Examples

To run the examples and generate outputs, invoke in the top-level directory:

```sh
make examples
```
