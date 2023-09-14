from . import parsing
from .. import utils
from invoke import task
import oyaml as yaml
import os

def _jobs_to_run(ctx):
    changed_files = utils.get_changed_files(ctx)
    with open("tasks/dynamic/FILEJOBS", "r") as f:
        yaml_content = yaml.load(f.read())
    jobs_to_run = []
    for file in changed_files:
        for key in yaml_content:
            if file.startswith(os.path.abspath(key)+os.sep):
                jobs_to_run.extend(yaml_content[key])
                break
    return jobs_to_run

@task
def dynamic_run(ctx, full_pipeline=False):
    extender = parsing.GitlabExtender(ctx, source_ci_file=".dynamic.yml")
    extender.run()
    extender.deps_graph.resolve_stage_dep()
    if not full_pipeline:
        extender.apply_jobs_data(extender.deps_graph.pipeline_jobs_to_run(_jobs_to_run(ctx)))

@task
def print_changed_files(ctx):
    print(utils.get_changed_files(ctx))


