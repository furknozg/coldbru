-- The campaign gains a short title (D-088): the pre-screen (screen@2.0.0) names the campaign in
-- the language of the brief, and the chat and the campaign list show it. NULL until the screen
-- answered; never blank and at most 60 characters when set.

-- +goose Up
ALTER TABLE campaign ADD COLUMN title text;
ALTER TABLE campaign ADD CONSTRAINT campaign_title_check CHECK (title IS NULL OR (title ~ '\S' AND char_length(title) <= 60));

-- +goose Down
ALTER TABLE campaign DROP CONSTRAINT campaign_title_check;
ALTER TABLE campaign DROP COLUMN title;
