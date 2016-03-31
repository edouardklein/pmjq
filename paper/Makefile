all: main.pdf

main.pdf: main.tex pipeline.pdf branch_av.pdf branch_shaky.pdf branch_petri.pdf branch_subset.pdf merge_av.pdf merge_shaky.pdf whole_pipeline.pdf
	latexmk -pdf

pipeline.pdf: pipeline_setup.sh
	pmjq_viz < pipeline_setup.sh  | dot -T pdf > pipeline.pdf

branch_av.pdf: branch_av_setup.sh
	pmjq_viz < branch_av_setup.sh  | dot -T pdf > branch_av.pdf

branch_shaky.pdf: branch_shaky.dot
	dot -T pdf < branch_shaky.dot > branch_shaky.pdf

branch_petri.pdf: branch_petri.dot
	dot -T pdf < branch_petri.dot > branch_petri.pdf

branch_subset.pdf: branch_subset.dot
	dot -T pdf < branch_subset.dot > branch_subset.pdf

merge_av.pdf: merge_av_setup.sh
	pmjq_viz < merge_av_setup.sh  | dot -T pdf > merge_av.pdf

merge_shaky.pdf: merge_shaky_setup.sh
	pmjq_viz < merge_shaky_setup.sh  | dot -T pdf > merge_shaky.pdf

whole_pipeline.pdf: whole_pipeline.txt
	grep -v -E '^#' < whole_pipeline.txt | pmjq_interactive  && pmjq_viz < setup.sh | dot -T pdf > whole_pipeline.pdf

whole_pipeline.png: whole_pipeline.txt
	grep -v -E '^#' < whole_pipeline.txt | pmjq_interactive  && pmjq_viz < setup.sh | dot -T png > whole_pipeline.png

clean:
	rm -f *.pdf main.aux main.bbl main.blg main.ent main.fdb_latexmk main.fls main.log \
main.out cleanup.sh launch.sh setup.sh
