from . import parsing
from .. import utils
from invoke import task
import oyaml as yaml
import os

def _jobs_to_run(ctx):
    changed_files = utils.get_changed_files(ctx)
    print("Changed files : ", changed_files)
    with open("tasks/dynamic/FILEJOBS", "r") as f:
        yaml_content = yaml.load(f.read(), Loader=yaml.SafeLoader)
    jobs_to_run = []
    for file in changed_files:
        for key in yaml_content:
            if file.startswith(key):
                jobs_to_run.extend(x for x in yaml_content[key] if x not in jobs_to_run)
                break
    print("Jobs needed to run : ", jobs_to_run)
    return jobs_to_run

@task
def dynamic_run(ctx, full_pipeline=False):
    jobs_to_run = _jobs_to_run(ctx)
    extender = parsing.GitlabExtender(ctx, source_ci_file=".gitlab-source.yml", output_folder=".dynamic")
    extender.run()
    extender.deps_graph.resolve_stage_dep()
    if not full_pipeline:
        extender.apply_jobs_data(extender.deps_graph.pipeline_jobs_to_run(jobs_to_run))

@task
def print_changed_files(ctx):
    print(utils.get_changed_files(ctx))


