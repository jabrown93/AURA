import { setRefsToFormItems } from "@/helper/download-modal/set-to-form-item";
import { ReturnErrorMessage } from "@/services/api-error-return";
import { CalendarDays, User } from "lucide-react";

import { useEffect, useState } from "react";

import { AssetImage } from "@/components/shared/asset-image";
import { hasEpisode, hasSeason, isInServer } from "@/components/shared/carousel-display";
import { DimmedBackground } from "@/components/shared/dimmed_backdrop";
import DownloadModal from "@/components/shared/download-modal";
import { ErrorMessage } from "@/components/shared/error-message";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Carousel, CarouselContent, CarouselItem } from "@/components/ui/carousel";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { H1, Lead } from "@/components/ui/typography";

import { isKometaSetId } from "@/lib/kometa";
import { log } from "@/lib/logger";
import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import type { BaseSetInfo, ImageFile, IncludedItem, SetRef } from "@/types/media-and-posters/sets";

export const ShowFullSetsDisplay: React.FC<{
  baseSetInfo: BaseSetInfo;
  posterSets: SetRef[];
  includedItems?: { [tmdb_id: string]: IncludedItem };
  dimNotFound?: boolean;
}> = ({ baseSetInfo, posterSets, includedItems = {}, dimNotFound = false }) => {
  const allPosters: ImageFile[] = [];
  const allBackdrops: ImageFile[] = [];
  const seasonPostersByShow: Record<string, ImageFile[]> = {};
  const titleCardsByShowAndSeason: Record<string, Record<number, ImageFile[]>> = {};

  if (!posterSets || posterSets.length === 0) {
    return <ErrorMessage error={ReturnErrorMessage<string>("No Poster Sets found")} />;
  }

  posterSets.forEach((item: SetRef) => {
    // All Posters are the poster and other posters
    for (const image of item.images || []) {
      if (image.type === "poster") {
        allPosters.push(image as ImageFile);
      } else if (image.type === "backdrop") {
        allBackdrops.push(image as ImageFile);
      } else if (image.type === "season_poster" || image.type === "special_season_poster") {
        const showTitle = includedItems?.[image.item_tmdb_id]?.mediux_info.title || "Unknown Show";

        if (!seasonPostersByShow[showTitle]) {
          seasonPostersByShow[showTitle] = [];
        }
        seasonPostersByShow[showTitle].push(image as ImageFile);
      } else if (image.type === "titlecard") {
        const showTitle = includedItems?.[image.item_tmdb_id]?.mediux_info.title || "Unknown Show";
        const seasonNumber = (image.season_number as number) || 0;

        if (!titleCardsByShowAndSeason[showTitle]) {
          titleCardsByShowAndSeason[showTitle] = {};
        }
        if (!titleCardsByShowAndSeason[showTitle][seasonNumber]) {
          titleCardsByShowAndSeason[showTitle][seasonNumber] = [];
        }
        titleCardsByShowAndSeason[showTitle][seasonNumber].push(image as ImageFile);
      }
    }
  });

  return (
    <div>
      <div className="p-2 lg:p-3">
        <div className="pb-4">
          {baseSetInfo.type !== "boxset" && (
            <ShowFullSetDetails baseSetInfo={baseSetInfo} posterSets={posterSets} includedItems={includedItems} />
          )}

          <Accordion
            type="multiple"
            className="w-full"
            defaultValue={["posters", "backdrops", "season-posters", "title-cards"]}
          >
            {/* All Posters */}
            {(allPosters.length > 0 || allBackdrops.length > 0) && (
              <AccordionItem value="posters">
                <AccordionTrigger>
                  {(() => {
                    const posterText = allPosters.length === 1 ? "Poster" : "Posters";
                    const backdropText = allBackdrops.length === 1 ? "Backdrop" : "Backdrops";

                    if (allBackdrops.length === 0) return posterText;
                    if (allPosters.length === 0) return backdropText;
                    return `${posterText} & ${backdropText}`;
                  })()}
                </AccordionTrigger>
                <AccordionContent>
                  <Carousel
                    opts={{
                      align: "start",
                      dragFree: true,
                      slidesToScroll: "auto",
                    }}
                    className="w-full"
                  >
                    <CarouselContent>
                      {(() => {
                        const getAvail = (tmdbId: string) => {
                          const found = isInServer(includedItems, tmdbId);
                          return Boolean(found && typeof found !== "boolean");
                        };

                        // Group by tmdbId so we can sort once and also support "backdrops-only" cases.
                        const tmdbIds = Array.from(
                          new Set<string>([
                            ...allPosters
                              .map((p) => (typeof p.item_tmdb_id === "string" ? p.item_tmdb_id : ""))
                              .filter((id) => id.trim() !== ""),
                            ...allBackdrops
                              .map((b) => (typeof b.item_tmdb_id === "string" ? b.item_tmdb_id : ""))
                              .filter((id) => id.trim() !== ""),
                          ])
                        );

                        tmdbIds.sort((a, b) => {
                          // available first (only meaningful when dimNotFound=true, but safe either way)
                          if (dimNotFound) {
                            const aAvail = getAvail(a) ? 1 : 0;
                            const bAvail = getAvail(b) ? 1 : 0;
                            if (aAvail !== bAvail) return bAvail - aAvail;
                          }

                          // tie-breaker: title (from included items) then tmdbId
                          const aTitle = includedItems?.[a]?.mediux_info?.title ?? "";
                          const bTitle = includedItems?.[b]?.mediux_info?.title ?? "";
                          const t = String(aTitle).localeCompare(String(bTitle));
                          if (t !== 0) return t;
                          return a.localeCompare(b);
                        });

                        const newestFirst = (x: ImageFile, y: ImageFile) =>
                          new Date(y.modified ?? 0).getTime() - new Date(x.modified ?? 0).getTime();

                        return tmdbIds.map((tmdbId) => {
                          const posters = allPosters
                            .filter((p) => p.item_tmdb_id === tmdbId)
                            .slice()
                            .sort(newestFirst);
                          const backdrops = allBackdrops
                            .filter((b) => b.item_tmdb_id === tmdbId)
                            .slice()
                            .sort(newestFirst);

                          const poster = posters[0];
                          const matchingBackdrop = backdrops[0];

                          if (!poster && !matchingBackdrop) return null;

                          const isAvailable = getAvail(tmdbId);

                          return (
                            <CarouselItem key={`${baseSetInfo.id ?? "set"}-posterbackdrop-${tmdbId}`}>
                              <div className="space-y-2">
                                {poster && (
                                  <AssetImage
                                    image={poster as unknown as ImageFile}
                                    imageType="mediux"
                                    aspect="poster"
                                    className={`w-full ${!isAvailable && dimNotFound ? "opacity-35" : ""}`}
                                    includedItems={includedItems}
                                    matchedToItem={isAvailable}
                                  />
                                )}

                                {matchingBackdrop && (
                                  <AssetImage
                                    image={matchingBackdrop as unknown as ImageFile}
                                    imageType="mediux"
                                    aspect="backdrop"
                                    className={`w-full ${!isAvailable && dimNotFound ? "opacity-35" : ""}`}
                                    includedItems={includedItems}
                                    matchedToItem={isAvailable}
                                  />
                                )}
                              </div>
                            </CarouselItem>
                          );
                        });
                      })()}
                    </CarouselContent>
                  </Carousel>
                </AccordionContent>
              </AccordionItem>
            )}

            {/* Season Posters by Show */}
            {Object.values(seasonPostersByShow).some((posters) => posters.length > 0) && (
              <AccordionItem value="season-posters">
                <AccordionTrigger>Season Posters</AccordionTrigger>
                <AccordionContent>
                  {Object.entries(seasonPostersByShow)
                    .filter(([, posters]) => posters.length > 0)
                    .map(([showTitle, posters]) => {
                      const sortedPosters = posters.slice().sort((a, b) => {
                        const aTmdb = typeof a.item_tmdb_id === "string" ? a.item_tmdb_id : "";
                        const bTmdb = typeof b.item_tmdb_id === "string" ? b.item_tmdb_id : "";

                        const aSeason = a.season_number ?? 0;
                        const bSeason = b.season_number ?? 0;

                        // available first (only when dimNotFound=true)
                        if (dimNotFound) {
                          const aAvail = aTmdb ? (hasSeason(includedItems, aTmdb, aSeason) ? 1 : 0) : 0;
                          const bAvail = bTmdb ? (hasSeason(includedItems, bTmdb, bSeason) ? 1 : 0) : 0;
                          if (aAvail !== bAvail) return bAvail - aAvail;
                        }

                        // season high -> low (Specials=0 will fall to the end)
                        if (aSeason !== bSeason) return bSeason - aSeason;

                        // newest modified first as a final tie-breaker
                        return new Date(b.modified ?? 0).getTime() - new Date(a.modified ?? 0).getTime();
                      });

                      return (
                        <div key={showTitle} className="mb-8">
                          <Lead className="mb-4">{showTitle}</Lead>
                          <Carousel
                            opts={{
                              align: "start",
                              dragFree: true,
                              slidesToScroll: "auto",
                            }}
                            className="w-full"
                          >
                            <CarouselContent>
                              {sortedPosters.map((poster) => (
                                <CarouselItem key={`season-poster-${poster.id}`}>
                                  <div className="space-y-2">
                                    <AssetImage
                                      key={poster.id}
                                      image={poster}
                                      imageType="mediux"
                                      aspect="poster"
                                      className={`w-full ${
                                        !hasSeason(
                                          includedItems,
                                          poster.item_tmdb_id as string,
                                          poster.season_number || 0
                                        ) && dimNotFound
                                          ? "opacity-35"
                                          : ""
                                      }`}
                                      includedItems={includedItems}
                                      matchedToItem={hasSeason(
                                        includedItems,
                                        poster.item_tmdb_id as string,
                                        poster.season_number || 0
                                      )}
                                    />
                                  </div>
                                </CarouselItem>
                              ))}
                            </CarouselContent>
                          </Carousel>
                        </div>
                      );
                    })}
                </AccordionContent>
              </AccordionItem>
            )}

            {/* Title Cards by Show and Season */}
            {Object.values(titleCardsByShowAndSeason).some((seasons) =>
              Object.values(seasons).some((cards) => cards.length > 0)
            ) && (
              <AccordionItem value="title-cards">
                <AccordionTrigger>Title Cards</AccordionTrigger>
                <AccordionContent>
                  {Object.entries(titleCardsByShowAndSeason)
                    .filter(([, seasons]) => Object.values(seasons).some((cards) => cards.length > 0))
                    // NEW: sort shows by availability first (when dimNotFound), then title
                    .sort(([aTitle, aSeasons], [bTitle, bSeasons]) => {
                      if (dimNotFound) {
                        const aAnyCard = Object.values(aSeasons).flat()[0];
                        const bAnyCard = Object.values(bSeasons).flat()[0];

                        const aTmdb =
                          typeof aAnyCard?.item_tmdb_id === "string" ? (aAnyCard.item_tmdb_id as string) : "";
                        const bTmdb =
                          typeof bAnyCard?.item_tmdb_id === "string" ? (bAnyCard.item_tmdb_id as string) : "";

                        const aAvail = aTmdb
                          ? isInServer(includedItems, aTmdb) && typeof isInServer(includedItems, aTmdb) !== "boolean"
                            ? 1
                            : 0
                          : 0;
                        const bAvail = bTmdb
                          ? isInServer(includedItems, bTmdb) && typeof isInServer(includedItems, bTmdb) !== "boolean"
                            ? 1
                            : 0
                          : 0;

                        if (aAvail !== bAvail) return bAvail - aAvail;
                      }
                      return String(aTitle).localeCompare(String(bTitle));
                    })
                    .map(([showTitle, seasons]) => (
                      <div key={showTitle} className="mb-8">
                        <Lead className="mb-4">{showTitle}</Lead>

                        <Accordion type="multiple" className="w-full">
                          {Object.entries(seasons)
                            .filter(([, cards]) => (cards as ImageFile[]).length > 0)
                            .sort(([aSeason, aCards], [bSeason, bCards]) => {
                              const aSeasonNum = Number(aSeason);
                              const bSeasonNum = Number(bSeason);

                              if (dimNotFound) {
                                const aFirst = aCards[0];
                                const bFirst = bCards[0];

                                const aTmdb = typeof aFirst?.item_tmdb_id === "string" ? aFirst.item_tmdb_id : "";
                                const bTmdb = typeof bFirst?.item_tmdb_id === "string" ? bFirst.item_tmdb_id : "";

                                const aAvail = aTmdb ? (hasSeason(includedItems, aTmdb, aSeasonNum) ? 1 : 0) : 0;
                                const bAvail = bTmdb ? (hasSeason(includedItems, bTmdb, bSeasonNum) ? 1 : 0) : 0;

                                if (aAvail !== bAvail) return bAvail - aAvail;
                              }

                              return bSeasonNum - aSeasonNum; // high -> low
                            })
                            .map(([seasonNumber, cards]) => {
                              const seasonNum = Number(seasonNumber);

                              // NEW: sort cards available-first (when dimNotFound), then latest episode, then newest modified
                              const sortedCards = (cards as ImageFile[]).slice().sort((a, b) => {
                                if (dimNotFound) {
                                  const aAvail =
                                    typeof a.item_tmdb_id === "string" && typeof a.episode_number === "number"
                                      ? hasEpisode(includedItems, a.item_tmdb_id, seasonNum, a.episode_number)
                                        ? 1
                                        : 0
                                      : 0;
                                  const bAvail =
                                    typeof b.item_tmdb_id === "string" && typeof b.episode_number === "number"
                                      ? hasEpisode(includedItems, b.item_tmdb_id, seasonNum, b.episode_number)
                                        ? 1
                                        : 0
                                      : 0;

                                  if (aAvail !== bAvail) return bAvail - aAvail;
                                }

                                const epDiff = (b.episode_number ?? 0) - (a.episode_number ?? 0);
                                if (epDiff !== 0) return epDiff;

                                return new Date(b.modified ?? 0).getTime() - new Date(a.modified ?? 0).getTime();
                              });

                              return (
                                <AccordionItem
                                  key={`${showTitle}-season-${seasonNumber}`}
                                  value={`${showTitle}-season-${seasonNumber}`}
                                >
                                  <AccordionTrigger>Season {seasonNumber}</AccordionTrigger>
                                  <AccordionContent>
                                    <Carousel
                                      opts={{
                                        align: "start",
                                        dragFree: true,
                                        slidesToScroll: "auto",
                                      }}
                                      className="w-full"
                                    >
                                      <CarouselContent>
                                        {sortedCards.map((card) => (
                                          <CarouselItem key={`title-card-${card.id}`}>
                                            <div className="space-y-2">
                                              <AssetImage
                                                image={card as unknown as ImageFile}
                                                imageType="mediux"
                                                aspect="backdrop"
                                                className={`w-full ${
                                                  !hasEpisode(
                                                    includedItems,
                                                    card.item_tmdb_id as string,
                                                    card.season_number || 0,
                                                    card.episode_number
                                                  ) && dimNotFound
                                                    ? "opacity-35"
                                                    : ""
                                                }`}
                                                includedItems={includedItems}
                                                matchedToItem={hasEpisode(
                                                  includedItems,
                                                  card.item_tmdb_id as string,
                                                  card.season_number || 0,
                                                  card.episode_number
                                                )}
                                              />
                                            </div>
                                          </CarouselItem>
                                        ))}
                                      </CarouselContent>
                                    </Carousel>
                                  </AccordionContent>
                                </AccordionItem>
                              );
                            })}
                        </Accordion>
                      </div>
                    ))}
                </AccordionContent>
              </AccordionItem>
            )}
          </Accordion>
        </div>
      </div>
    </div>
  );
};

const ShowFullSetDetails: React.FC<{
  baseSetInfo: BaseSetInfo;
  posterSets: SetRef[];
  includedItems?: { [tmdb_id: string]: IncludedItem };
}> = ({ baseSetInfo, posterSets, includedItems }) => {
  const [backdropURL, setBackdropURL] = useState("");

  const [mediuxURL, setMediuxURL] = useState<string>("");
  const { status, hasHydrated } = useOnboardingStore();
  const mediuxSiteLink = status?.mediux_site_link || "https://mediux.io";

  useEffect(() => {
    if (!hasHydrated) return;
    if (!baseSetInfo.id || !baseSetInfo.type) {
      setMediuxURL("");
      return;
    }

    if (mediuxSiteLink.endsWith("mediux.pro")) {
      // https://mediux.pro/[itemType]s/tmdbID
      setMediuxURL(`${mediuxSiteLink}/sets/${baseSetInfo.id}`);
      return;
    } else if (mediuxSiteLink.endsWith("mediux.io")) {
      // https://mediux.io/[itemType]/tmdbID
      setMediuxURL(`${mediuxSiteLink}/${baseSetInfo.type}/${baseSetInfo.id}`);
      return;
    }
  }, [hasHydrated, mediuxSiteLink, baseSetInfo.id, baseSetInfo.type]);

  const showDateModified = useUserPreferencesStore((state) => state.showDateModified);
  const setShowDateModified = useUserPreferencesStore((state) => state.setShowDateModified);

  // Construct the backdrop URL
  // If there is only one set, then check to see if there is a backdrop image in that set
  // If there are multiple sets, randomly select one of the backdrops from those sets
  // If there are no backdrops in the sets, randomly select one from the included items' TMDB backdrops
  useEffect(() => {
    let selectedBackdropURL = "";

    const allBackdrops: ImageFile[] = [];

    posterSets.forEach((item) => {
      item.images.forEach((image) => {
        if (image.type === "backdrop") {
          allBackdrops.push(image);
        }
      });
    });

    if (allBackdrops.length > 0) {
      // Randomly select one of the backdrops from the sets
      const randomIndex = Math.floor(Math.random() * allBackdrops.length);
      selectedBackdropURL = `/api/images/mediux/item?asset_id=${allBackdrops[randomIndex].id}&modified_date=${allBackdrops[randomIndex].modified}&quality=optimized`;
    } else {
      // No backdrops in the sets -> pick a TMDB backdrop for ANY unique item_tmdb_id found in posterSets images
      const tmdbIDs = new Set<string>();

      for (const ps of posterSets || []) {
        for (const img of ps.images || []) {
          const tmdb = img.item_tmdb_id;
          if (typeof tmdb === "string" && tmdb.trim() !== "") {
            tmdbIDs.add(tmdb);
          }
        }
      }

      const includedBackdrops = Array.from(tmdbIDs)
        .map((tmdbId) => includedItems?.[tmdbId]?.mediux_info?.tmdb_backdrop_path)
        .filter((p): p is string => typeof p === "string" && p.trim() !== "")
        .map((p) => `https://image.tmdb.org/t/p/original${p}`);

      if (includedBackdrops.length > 0) {
        const randomIndex = Math.floor(Math.random() * includedBackdrops.length);
        selectedBackdropURL = includedBackdrops[randomIndex];
      }
    }

    setBackdropURL(selectedBackdropURL);
  }, [posterSets, includedItems]);

  return (
    <>
      {/* Backdrop Background */}
      {backdropURL && <DimmedBackground backdropURL={backdropURL} />}

      {/* Title */}
      <div className="flex flex-col pt-40 justify-end items-center text-center lg:items-start lg:text-left">
        <H1
          className="mb-1"
          onClick={() => {
            log("INFO", "ShowFullSetDetails", "Return to Set Details", "Set Details Clicked", {
              baseSetInfo,
            });
          }}
        >
          {baseSetInfo.title}
        </H1>
      </div>

      {/* Set Author */}
      <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center text-white gap-4 tracking-wide mt-4">
        <div className="flex items-center gap-2">
          <Badge
            className="flex items-center text-sm hover:text-white transition-colors hover:brightness-120 cursor-pointer active:scale-95"
            onClick={(e) => {
              e.stopPropagation();
              window.location.href = `/user/${baseSetInfo.user_created}`;
            }}
          >
            <Avatar className="rounded-lg mr-1 w-4 h-4">
              <AvatarImage src={`/api/images/mediux/avatar?username=${baseSetInfo.user_created}`} className="w-4 h-4" />
              <AvatarFallback className="">
                <User className="w-4 h-4" />
              </AvatarFallback>
            </Avatar>
            {baseSetInfo.user_created}
          </Badge>
          {!isKometaSetId(baseSetInfo.id) && (
            <Badge
              className="flex items-center text-sm hover:text-white transition-colors hover:brightness-120 cursor-pointer active:scale-95"
              onClick={(e) => {
                e.stopPropagation();
                window.open(`${mediuxURL}`, "_blank");
              }}
            >
              View on MediUX
            </Badge>
          )}
        </div>

        {/* Season/Episode Information */}
        {baseSetInfo.type === "show" &&
          (() => {
            let seasonPosterCount = 0;
            let titleCardCount = 0;

            for (const set of posterSets || []) {
              for (const image of set.images || []) {
                if (image.type === "season_poster" || image.type === "special_season_poster") {
                  seasonPosterCount += 1;
                } else if (image.type === "titlecard") {
                  titleCardCount += 1;
                }
              }
            }

            if (seasonPosterCount === 0 && titleCardCount === 0) return null;

            return (
              <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide">
                <Lead className="flex items-center text-md text-primary-dynamic">
                  {seasonPosterCount > 0 && (
                    <>
                      {seasonPosterCount} {seasonPosterCount === 1 ? "Season Poster" : "Season Posters"}
                    </>
                  )}

                  {titleCardCount > 0 && (
                    <>
                      {seasonPosterCount > 0 ? " with " : ""}
                      {titleCardCount} {titleCardCount === 1 ? "Title Card" : "Title Cards"}
                    </>
                  )}
                </Lead>
              </div>
            );
          })()}

        {/* Movies Information 
				Get a count of total number of posters and backdrops for movies in the set
				Only display if posters or backdrops exist
				*/}
        {baseSetInfo.type === "movie" &&
          (() => {
            let posterCount = 0;
            let backdropCount = 0;

            for (const set of posterSets || []) {
              for (const image of set.images || []) {
                if (image.type === "poster") {
                  posterCount += 1;
                } else if (image.type === "backdrop") {
                  backdropCount += 1;
                }
              }
            }

            if (posterCount === 0 && backdropCount === 0) return null;

            return (
              <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide">
                <Lead className="flex items-center text-md text-primary-dynamic">
                  {posterCount > 0 && (
                    <>
                      {posterCount} {posterCount === 1 ? "Poster" : "Posters"}
                    </>
                  )}

                  {backdropCount > 0 && (
                    <>
                      {posterCount > 0 ? " with " : ""}
                      {backdropCount} {backdropCount === 1 ? "Backdrop" : "Backdrops"}
                    </>
                  )}
                </Lead>
              </div>
            );
          })()}

        {/* Download Button */}
        <div className="ml-auto">
          <DownloadModal baseSetInfo={baseSetInfo} formItems={setRefsToFormItems(posterSets, includedItems || {})} />
        </div>
      </div>

      <div className="flex justify-center lg:justify-start items-center gap-4 mt-4">
        <Label
          htmlFor="overlay-date-switch"
          className="flex items-center text-sm cursor-pointer hover:text-white transition-colors hover:brightness-120"
        >
          <CalendarDays className="w-4 h-4 mr-1" />
          Show Modified Date
        </Label>
        <Switch
          id="overlay-date-switch"
          checked={showDateModified}
          onCheckedChange={() => setShowDateModified(!showDateModified)}
          aria-label="Toggle bookmark"
        />
      </div>
    </>
  );
};
