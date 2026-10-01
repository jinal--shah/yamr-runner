package cli

const overridesHelp = `
Example:

/repo/
├── prod/
│   ├── .yamr.yaml # only has yamr-runner.env 
│   └── subdir/
│       └── .yamr.yaml # has 'source-label' "foo" and overrides container cmd_opts
├── stag/
│   └── .yamr.yaml # has 'sources' list and sets yamr-runner.action.run.docker.cmd_sources to $yamr_sources$
├── sources/
│   ├── one.yaml
│   └── two.yaml
├── yamr-runner-defaults.yaml # contains all the defaults of what to run and what to do on_fail
└── yamr-source-labels.yaml  # contains list "foo" of sources

# /repo/yamr-runner-defaults.yaml
yamr-runner:
  env:
    ENTITY: foo
    STACK: dev
  action:
    pre_run:
      mv_to_tmp:
        path: /$action_dir$/.generated
      mkdir:
        path: /$action_dir$/.generated
    run:
      docker:
        image: jinal--shah/yamr:stable
        entrypoint:
          - yamr
        user_group: $uid_me$:$gid_me$
        work_dir: /$yamr_sources_dir$ # run yamr from /$yamr_sources_dir
        mounts:
          - /$repo_root$:/$repo_root$ # mount the git repo with the action file in it
		  - /$yamr_sources_dir:/$yamr_sources_dir # mount the source files you want to yamr
        cmd_opts: [] # no options needed for yamr container CMD
	on_fail:
      run:
        docker:
          image: jinal--shah/yamr:stable
          entrypoint:
            - yamr
          user_group: $uid_me$:$gid_me$
          work_dir: /$yamr_sources_dir$ # run yamr from /$yamr_sources_dir
          mounts:
            - /$repo_root$:/$repo_root$ # mount the git repo with the action file in it
  		  - /$yamr_sources_dir:/$yamr_sources_dir # mount the source files you want to yamr
          cmd_opts: ["-d", "comments=true", "-u", "-t", "-o", "/$action_dir/.yamr-debug/stdout.log", "--"]
          cmd_sources: $yamr_sources_from_label$
        stderr: /$action_dir/.yamr-debug/stderr.log

# /repo/yamr-source-labels.yaml
my-vpc-config:
- $env.STACK$.yaml
- vpc/$env.ENTITY/$env.FOO$.yaml


# /repo/prod/.yamr.yaml
yamr-runner:
  env:
    STACK: prod # will overwrite yamr-runner.env.STACK from yamr-runner-defaults.yaml

# /repo/prod/subdir/.yamr.yaml
trigger-yamr-runner: true # this is the file that will trigger an action run
yamr-runner:
  source-label: my-vpc-config
  env:
    FOO: bar

A container action will be triggered from dir /prod/subdir because it contains
    trigger-yamr-runner: true

Assuming the user's uid:gid is 501:501, it will run pre_run steps equivalent to:

    # from yamr-runner-defaults.yaml, yamr-runner.action.pre_run[0] - mv_to_tmp /$action_dir$/.generated
    if [[ -e /home/foo/subdir/.generated ]]; then
        tmp_dir=$(mktmp); mkdir -p $tmp/dir/home/foo/subdir
        mv /home/foo/subdir/.generated $tmp_dir/home/foo/subdir/
    fi

    # from yamr-runner-defaults.yaml, yamr-runner.action.pre_run[1] - mkdir /$action_dir$/.generated
    mkdir -p /home/foo/subdir/.generated

    docker run -t --rm
        --entrypoint yamr # yamr-runner-defaults.yaml
        -u 501:501        # yamr-runner-defaults.yaml $uid_me$:$gid_me$
        -e ENTITY=foo # yamr-runner-defaults.yaml yamr-runner.env.ENTITY
        -e STACK=prod # /repo/prod/.yamr.yaml yamr-runner.env.STACK
        -e FOO=bar    # /repo/prod/subdir/.yamr.yaml yamr-runner.env.FOO
        -w /sources   # yamr-runner-defaults.yaml yamr-runner.action.run.docker.work_dir
        -v /repo:/repo       # yamr-runner-defaults.yaml yamr-runner.action.run.docker.mounts[0]
        -v /sources:/sources # yamr-runner-defaults.yaml yamr-runner.action.run.docker.mounts[1]
        --entrypoint yamr    # yamr-runner-defaults.yaml yamr-runner.action.run.docker.entrypoint
            jinal--shah/yamr:stable # yamr-runner-defaults.yaml yamr-runner.action.run.docker.image
                prod.yaml           # via sources-label my-vpc-config in /repo/yamr-source-labels.yaml, envvars replaced
                vpc/foo/bar.yaml    # via sources-label my-vpc-config in /repo/yamr-source-labels.yaml, envvars replaced 

`
