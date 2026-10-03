* bug - can't use $..$ vars in main config file for yamr-sources-dir and yamr-source-labels
    should be able to.
* bug:
    * boundary should NOT be the run_dir.
        It should be the directory path up to and including the repo root so we can inherit still
        Otherwise we can't run from a single action file and inherit ...
