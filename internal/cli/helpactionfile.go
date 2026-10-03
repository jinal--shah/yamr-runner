package cli

const actionFileHelp = `yamr-runner action file

Action files are named:

  .yamr.yaml

Most yamr-runner.action settings override defaults from the main
yamr-runner configuration file.

See the defaults available there with:

  yamr-runner -h config-file

However these yaml paths should only be set in the action file, not
the main config:

* trigger-yamr-runner: (true | false) - run a container action from this dir
* yamr-runner.sources-label
* yamr-runner.sources # you'll only use this instead of, or to override sources-label

Kitchen-sink example:

  # trigger-yamr-runner:
  # If true, this means run the desired container from this directory.
  # Action files in any subdirs of this directory will be ignored.
  trigger-yamr-runner: true # optional, default false
  
  # yamr-runner:
  # any overrides from the defaults you set up in the main config file
  # Run yamr-runner -h config-file for more information
  # This entire map is optional in an action file if the config file
  # contains all the required data to run the container
  yamr-runner:

    # SOURCES-LABEL || SOURCES:
    # you must supply one, either here or in the config file.
    # If both are supplied, sources takes precedence over sources-label

    # yamr-runner.sources-label:
    # A top level key in your yamr-source-labels yaml
    # to a list of the sources to pass to yamr as the
    # final arguments to docker CMD.
    # Set yaml-runner.action.run.docker.cmd_sources to
    # $yamr_sources_from_label$ to have them included.
    sources-label: customer

    # Alternatively, you can provide yamr-runner.sources, an inline list 
    # and set yaml-runner.action.run.docker.cmd_sources to $yamr_sources$
    # sources:
    #   - customer/base.yaml
    #   - customer/production.yaml

    # yamr-runner.env:
    # environment variables to set in the docker container
    # and can also be used to resolve dynamic config values
    # e.g. $env.FOO$ will be replaced with the resolved value
    # of the FOO environment variable you are passing the container.
    # The env map specified here is merged into any env map
    # specified in a .yamr.yaml file in any direct ancestor directory
    env:
      EXAMPLE_ENV: action-specific-value

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
	      # REQUIRED: action file must set this or inherit it.
          # If using source-label, set to:
          #     $yamr_sources_from_label$
          # If using sources, set to:
          #     $yamr_sources$
          cmd_sources: $yamr_sources_from_label$

      # yamr-runner.action.on_fail:
      # Contains the pre_run, and docker command to run
      # when yamr-runner.action.run.docker command fails
      on_fail:
        # pre_run: Same map keys as for yamr-runner.action.pre_run
        pre_run:
        # run: Same map keys as for yamr-runner.action.run
		run:
`
