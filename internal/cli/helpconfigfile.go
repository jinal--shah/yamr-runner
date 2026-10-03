package cli

const configFileHelp = `yamr-runner configuration file

The configuration file supplies defaults which may be overridden by
each action file in turn along a directory path, until and including
the action file that triggers a container action.

Only the yamr-runner map can be overridden in action files.
i.e. other top level keys in the config file aren't supported in action files.

Example:

  # yamr-sources-dir:
  # Optional when --yamr-sources-dir or YAMR_SOURCES_DIR is supplied.
  # Directory containing YAMR source files.
  # The only token in this value is $this_dir$ (the dir containing the config file)
  yamr-sources-dir: /path/to/yamr-sources

  # yamr-source-labels:
  # Optional when --yamr-source-labels or YAMR_SOURCE_LABELS is supplied.
  # File containing source-label mappings.
  # The only token in this value is $this_dir$ (the dir containing the config file)
  yamr-source-labels: /path/to/yamr-source-labels.yaml

  # yamr-runner:
  # This map is the defaults that can be overriden by directory specific action files
  yamr-runner:
    # yamr-runner.env:
    # Environment variables, to pass to the docker container run by an action file
    # AND can also be used to resolve dynamic config values in yamr-runner config
    # and in the yamr-source-labels file.
    # The env map specified here is effectively the default env map passed to
    # any action file.
    # i.e. action-file env values can add to, or override this map.
    env:
      EXAMPLE_ENV: example-value

    # yamr-runner.action
    # the operations to perform, and what to do on failure
    action:

      # yamr-runner.action.pre_run:
      # Optional operations performed before the docker container runs.
      # Available operations:
      # mv_to_tmp: moves a path to a temp location, skipped if path
      #               does not exist.
      # mkdir: create a dir (and missing ancestor dirs)
      pre_run:
        - mv_to_tmp:
            path: /$action_dir$/.generated

        - mkdir: # create a dir (and missing ancestor dirs)
            path: /$action_dir$/.generated
            chmod: "0755" # optional, default "0755"
            chown: $uid_me$:$gid_me$ # optional, default is eid:gid

      # yamr-runner.action.run:
      # contains the docker or host action to run
      # currently only docker is supported.
      run:
        # yamr-runner.action.run.docker:
        # The arguments that are passed to docker run --rm.
        # Containers are expected to run as oneshot and not daemons
        docker:
          # docker <repo>/<image>:<tag>
          image: propero/yamr:candidate

          # --entrypoint
          entrypoint:
            - yamr

          # --user
          user_group: $uid_me$:$gid_me$

          # -w
          work_dir: /$yamr_sources_dir$

          # -v - specify one mapping per list item
          mounts:
            - /$repo_root$:/$repo_root$

          # args to pass the image BEFORE yamr source files list
          cmd_opts:
            - run

          # yamr-runner.action.run.docker.cmd_sources:
		  # MUST resolve for the triggering action_file, so you can put
		  # the common case in this file, or in any action file
		  # the triggering action file inherits.
          # If using source-label, set to:
          #     $yamr_sources_from_label$
          # If using sources, set to:
          #     $yamr_sources$
          cmd_sources: $yamr_sources_from_label$

      # yamr-runner.action.on_fail:
      # Contains the pre_run, and docker command to run
      # when yamr-runner.action.run.docker command fails
      on_fail:
        # pre_run: Same as for yamr-runner.action.pre_run
        pre_run:
        # run: Same as for yamr-runner.action.run with some additions, see below:
		run:
          # additional options for yamr-runner.action.on_fail.run:
          # stdout, stderr: path to send on_fail container output
          # if setting is null, then the container sends that stream whereever
          # it is configured to send it.
          # if setting is omitted, will write to /$action_dir$/.yamr-debug/<stream>.log
          stdout: /$action_dir$/.yamr-debug/stdout.log
          stderr: /$action_dir$/.yamr-debug/stderr.log

For an annotated action file, see:

  yamr-runner -h action-file
`
