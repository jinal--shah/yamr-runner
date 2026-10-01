package cli

const examplesHelp = `yamr-runner examples

A typical setup uses:

  yamr-runner-defaults.yaml
  yamr-source-labels.yaml
  .yamr.yaml

Example command:

  yamr-runner \
    --config ./yamr-runner-defaults.yaml \
    --yamr-sources-dir ./sources \
    --yamr-source-labels ./yamr-source-labels.yaml

yamr-runner-defaults.yaml:

  yamr-runner:
    action:
      run:
        docker:
          image: propero/yamr:candidate
          entrypoint:
            - yamr
          user_group: $uid_me$:$gid_me$
          work_dir: /$yamr_sources_dir$
          mounts:
            - /$repo_root$:/$repo_root$
          cmd_opts:
            - run
          cmd_sources: $yamr_sources_from_label$

yamr-source-labels.yaml:

  customer:
    - customer/base.yaml
    - customer/production.yaml

.yamr.yaml:

  trigger-yamr-runner: true

  yamr-runner:
    sources-label: customer

The action file is merged over the defaults from the configuration file.

For complete annotated examples, see:

  yamr-runner -h config-file
  yamr-runner -h action-file
`
