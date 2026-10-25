PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
CREATE TABLE VERSION (
		version INTEGER NOT NULL
	);
INSERT INTO VERSION VALUES(6);
CREATE TABLE AUTH (
		token_secret TEXT NOT NULL
	);
CREATE TABLE MediaItems (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	tmdb_id TEXT NOT NULL,
	library_title TEXT NOT NULL,
	rating_key TEXT NOT NULL,
	type TEXT NOT NULL CHECK (type IN ('movie','show')),
	title TEXT NOT NULL,
	year INTEGER NOT NULL,
	on_server INTEGER NOT NULL DEFAULT 0 CHECK (on_server IN (0,1)),
	UNIQUE (tmdb_id, library_title)
);
INSERT INTO MediaItems VALUES(1,'1399','TV Shows','100','show','Game of Thrones',2011,1);
INSERT INTO MediaItems VALUES(2,'1396','TV Shows','101','show','Breaking Bad',2008,1);
INSERT INTO MediaItems VALUES(3,'603','Movies','200','movie','The Matrix',1999,1);
CREATE TABLE Movies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL UNIQUE,
    path TEXT NOT NULL,
    size INTEGER NOT NULL,
    duration INTEGER NOT NULL,
    FOREIGN KEY (media_item_id) REFERENCES MediaItems(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE
);
CREATE TABLE Series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL UNIQUE,
    season_count INTEGER,
    episode_count INTEGER,
    location TEXT,
    FOREIGN KEY (media_item_id) REFERENCES MediaItems(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE
);
CREATE TABLE Seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL,
	rating_key TEXT NOT NULL,
    season_number INTEGER NOT NULL,
    episode_count INTEGER,
    FOREIGN KEY (series_id) REFERENCES Series(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    UNIQUE (series_id, season_number)
);
CREATE TABLE Episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    season_id INTEGER NOT NULL,
	rating_key TEXT NOT NULL,
    episode_number INTEGER NOT NULL,
    title TEXT,
    path TEXT NOT NULL,
    size INTEGER NOT NULL,
    duration INTEGER NOT NULL,
    FOREIGN KEY (season_id) REFERENCES Seasons(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    UNIQUE (season_id, episode_number)
);
CREATE TABLE PosterSets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id TEXT NOT NULL UNIQUE,
	type TEXT NOT NULL CHECK (type IN ('show','movie','collection')),
    title TEXT NOT NULL,
    user TEXT NOT NULL,
    date_created DATETIME,
    date_updated DATETIME
);
INSERT INTO PosterSets VALUES(1,'s1','show','GoT set','alice','2026-01-01','2026-01-02');
INSERT INTO PosterSets VALUES(2,'s2','show','BB set','bob','2026-01-01','2026-01-02');
CREATE TABLE ImageFiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    poster_set_id INTEGER NOT NULL,

    -- Which item inside the set this image belongs to (required for collection sets)
    item_tmdb_id TEXT NOT NULL,

    image_id TEXT NOT NULL,
    image_type TEXT NOT NULL CHECK (image_type IN ('poster','backdrop','season_poster','titlecard')),
    image_last_updated DATETIME NOT NULL,
    image_season_number INTEGER,
    image_episode_number INTEGER,

    FOREIGN KEY (poster_set_id) REFERENCES PosterSets(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE,

    -- Same image_id can exist multiple times in a set if it belongs to different items (e.g., collections)
    UNIQUE (poster_set_id, image_id, item_tmdb_id)
);
CREATE TABLE SavedItems (
    tmdb_id TEXT NOT NULL,
    library_title TEXT NOT NULL,
    poster_set_id INTEGER NOT NULL,

    -- Per MediaItem/Set toggles
    poster_selected INTEGER NOT NULL DEFAULT 0 CHECK (poster_selected IN (0,1)),
    backdrop_selected INTEGER NOT NULL DEFAULT 0 CHECK (backdrop_selected IN (0,1)),
    season_poster_selected INTEGER NOT NULL DEFAULT 0 CHECK (season_poster_selected IN (0,1)),
    special_season_poster_selected INTEGER NOT NULL DEFAULT 0 CHECK (special_season_poster_selected IN (0,1)),
    titlecard_selected INTEGER NOT NULL DEFAULT 0 CHECK (titlecard_selected IN (0,1)),

    autodownload INTEGER NOT NULL DEFAULT 0 CHECK (autodownload IN (0,1)),
	auto_add_new_collection_items INTEGER NOT NULL DEFAULT 0 CHECK (auto_add_new_collection_items IN (0,1)),
	force_preload_missing INTEGER NOT NULL DEFAULT 0 CHECK (force_preload_missing IN (0,1)),
    last_downloaded DATETIME NOT NULL,

    PRIMARY KEY (tmdb_id, library_title, poster_set_id),

    FOREIGN KEY (poster_set_id) REFERENCES PosterSets(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE,

    FOREIGN KEY (tmdb_id, library_title) REFERENCES MediaItems(tmdb_id, library_title)
        ON DELETE CASCADE
        ON UPDATE CASCADE
) WITHOUT ROWID;
INSERT INTO SavedItems VALUES('1396','TV Shows',2,1,0,0,0,0,0,0,0,'2026-02-01 00:00:00');
INSERT INTO SavedItems VALUES('1399','TV Shows',1,1,0,1,0,1,1,0,1,'2026-02-01 00:00:00');
CREATE TABLE IgnoredItems (
    tmdb_id TEXT NOT NULL,
    library_title TEXT NOT NULL,

    -- 'always' = persist until user un-ignores
    -- 'until-set-available'   = cleared by cron job when a set becomes available
	-- 'until-new-set-available' = never cleared by cron job, but user notified when a new set becomes available and given the option to clear the ignore manually
    mode TEXT NOT NULL CHECK (mode IN ('always','until-set-available','until-new-set-available')),
	
	-- Sets that currently available for this item (array stored as JSON string)
	current_sets TEXT NOT NULL DEFAULT '[]',

    PRIMARY KEY (tmdb_id, library_title)
) WITHOUT ROWID;
INSERT INTO IgnoredItems VALUES('603','Movies','always','[]');
PRAGMA writable_schema=ON;
CREATE TABLE IF NOT EXISTS sqlite_sequence(name,seq);
DELETE FROM sqlite_sequence;
CREATE INDEX idx_seasons_series_id ON Seasons(series_id);
CREATE INDEX idx_episodes_season_id ON Episodes(season_id);
CREATE INDEX idx_imagefiles_poster_set_id ON ImageFiles(poster_set_id);
CREATE INDEX idx_imagefiles_set_type ON ImageFiles(poster_set_id, image_type);
CREATE INDEX idx_imagefiles_item_tmdb_id ON ImageFiles(item_tmdb_id);
CREATE INDEX idx_imagefiles_item_tmdb_type ON ImageFiles(item_tmdb_id, image_type);
CREATE INDEX idx_saveditems_poster_set_id ON SavedItems(poster_set_id);
CREATE INDEX idx_saveditems_item ON SavedItems(tmdb_id, library_title);
CREATE INDEX idx_ignoreditems_mode ON IgnoredItems(mode);
PRAGMA writable_schema=OFF;
COMMIT;
