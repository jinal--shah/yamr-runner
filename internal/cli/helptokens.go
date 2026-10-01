package cli

const tokensHelp = `yamr-runner tokens

Special tokens may be used in yamr-runner configuration, action YAML and the yaml source labels file.

Tokens are resolved by yamr-runner before an action is executed.

PATH TOKENS

Path tokens return canonical absolute paths with the leading and trailing
slash removed.

So if you want an absolute path, don't forget to add the leading '/'
e.g.
  /$repo_root$/some/path
  /$action_dir$/.generated

$repo_root$

  The path to the git repo dir containing the action files.

  So if action files are under a repo at /home/me/foo

    /$repo_root$/shared # resolves to /home/me/foo/shared


$this_dir$

  The directory containing the YAML file currently being processed.

  The directory of the YAML file that contains $this_dir$.

  If your action file at /home/me/foo/bar/.yamr.yaml includes
      env:
      FOO_DIR: /$this_dir$/templates # resolves to /home/me/foo/bar/templates

$this_dir_rel_to_repo_root$

  The directory of the YAML file containing $this_dir_rel_to_repo_root$, relative to $repo_root$

  Empty string if the YAML file containing $this_dir_rel_to_repo_root$ is in the $repo_root$ dir.

  Example repository:

    /repo
      services/
        example/
          .yamr.yaml

    # .yamr.yaml
    env:
      SRC_ACTION_FILE: $this_dir_rel_to_repo_root$ # resolves to services/example

$run_dir$

  The directory in which yamr-runner was invoked.

  yamr-runner searches for actions beneath this directory.


$yamr_sources_dir$

  The configured YAMR sources directory.

  This comes from --yamr-sources-dir, YAMR_SOURCES_DIR, or the
  yamr-sources-dir configuration value according to normal option
  precedence.

$action_dir$

  The directory containing the action file being executed.

  It is particularly useful for pre-run operations and action-specific
  output paths.

  Example:

    # /foo/bar/.yamr.yaml
    trigger-yamr-runner: true

    # in parent dir, /foo/.yamr.yaml
    env:
      RESULTS_DIR=/$action_dir$/.generated # resolves to /hom/me
      # ^^^ when /foo/bar/.yamr.yaml container runs, RESULTS_DIR will be /foo/bar/.generated

USER AND GROUP TOKENS

$uid_me$
  The yamr-runner user's effective uid
  Commonly used for Docker user/group or ownership configuration so generated files on mounted volumes
  are owned by your own user.

  Typically used for yamr-runner.action.run.docker.user_group and yamr-runner.on_fail.run.docker.user_group

    user_group: $uid_me$:$gid_me$

$gid_me$
  The yamr-runner user's effective gid
  Like $uid_me$ but for group


ENVIRONMENT TOKENS

$env.NAME$

  Resolves NAME from the environment available to the action.

  Example:

    $env.HOME$

  or embedded in a larger value:

    /$env.OUTPUT_DIR$/generated

  Environment values are built from the process environment and
  yamr-runner.env configuration.

  Values configured in the env section in yamr-runner config and action files override
  values inherited from the process.

  An empty YAML string is a valid value.

  A null value removes the variable from the map. This also prevents a value inherited
  from the process environment from being used.

  Example:

    yamr-runner:
      env:
        OUTPUT_DIR: /tmp/output
        EMPTY_VALUE: ""
        REMOVE_ME: null

  Referencing an environment variable which is not defined is an error.


SOURCE TOKENS

Source tokens are different from the other tokens. They represent a YAML
sequence of source arguments rather than a string.

Because of this, a source token MUST constitute the entire YAML value.
It cannot be embedded inside another string.


$yamr_sources_from_label$

  Expands to the list of sources selected by sources-label.

  Normally used as:

    yamr-runner:
      sources-label: production

      action:
        run:
          docker:
            cmd_sources: $yamr_sources_from_label$

  At execution time, cmd_sources becomes the complete sequence of source
  arguments associated with the selected label.


$yamr_sources$

  Expands to the list supplied by an action's inline sources field.

  Example:

    yamr-runner:
      sources:
        - services/example/base.yaml
        - services/example/production.yaml

      action:
        run:
          docker:
            cmd_sources: $yamr_sources$

  When inline sources are used, cmd_sources must use $yamr_sources$.


SOURCE TOKEN RESTRICTIONS

$yamr_sources$ and $yamr_sources_from_label$ are lists.

They must be used as the entire yaml value, not as a substring.

e.g. this is not valid:

  cmd_sources: prefix-$yamr_sources$

TOKEN RESOLUTION

Unknown tokens - anything matching regex \$[_\w]+\$ that is not one of the known tokens will cause yamr-runner to fail
before launching any containers.

COMMON EXAMPLES

Run the container as the current user:

  user_group: $uid_me$:$gid_me$

Mount the repository at the same absolute path inside the container:

  mounts:
    - /$repo_root$:/$repo_root$

Use the configured sources directory as the container working directory:

  work_dir: /$yamr_sources_dir$

Create an action-specific directory:

  pre_run:
    - mkdir:
        path: /$action_dir$/.generated
        chmod: "0755"
        chown: $uid_me$:$gid_me$

Use an environment value:

  env:
    ENVIRONMENT: production

  action:
    run:
      docker:
        cmd_opts:
          - --environment
          - $env.ENVIRONMENT$

Use sources selected by a label:

  sources-label: production

  action:
    run:
      docker:
        cmd_sources: $yamr_sources_from_label$

Use sources specified directly by an action:

  sources:
    - service/base.yaml
    - service/production.yaml

  action:
    run:
      docker:
        cmd_sources: $yamr_sources$
`
