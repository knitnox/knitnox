-- What a person sees of the AI's work in a chat. Both are about display only:
-- nothing that is sent to the model changes.
--   show_thinking  the fold-away "Thinking" block of models that share their reasoning
--   show_tools     every tool call and result as a row of its own; off = one
--                  quiet line per answer ("Used 2 tools") that opens on a click
ALTER TABLE users ADD COLUMN show_thinking INTEGER NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN show_tools INTEGER NOT NULL DEFAULT 0;
