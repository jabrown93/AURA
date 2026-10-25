import { setRefsToFormItems } from "@/helper/download-modal/set-to-form-item";
import { GetMediaItemDetails } from "@/services/mediaserver/get-media-item-details";
import type { CreatorBoxset } from "@/services/mediux/creator-sets";

import { useEffect, useRef, useState } from "react";

import DownloadModal from "@/components/shared/download-modal";
import { MediaCarousel } from "@/components/shared/media-carousel";
import { ShowFullSetsDisplay } from "@/components/shared/show-full-set";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";

import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { IncludedItem, SetRef } from "@/types/media-and-posters/sets";

export const RenderShowAndCollectionDisplay = ({
  set,
  includedItems,
}: {
  set: SetRef;
  includedItems?: { [tmdb_id: string]: IncludedItem };
}) => {
  // Find the first valid media item
  let initialMediaItem: MediaItem | undefined = undefined;
  let initialMediaKey: string | undefined = undefined;
  if (includedItems) {
    for (const itemId of set.item_ids) {
      const includedItem = includedItems[itemId];
      if (includedItem && includedItem.media_item.title && includedItem.media_item.rating_key) {
        initialMediaItem = includedItem.media_item;
        initialMediaKey = itemId;
        break;
      }
    }
  }

  // Local state for media item and includedItems
  const [mediaItem, setMediaItem] = useState<MediaItem | undefined>(initialMediaItem);
  const [localIncludedItems, setLocalIncludedItems] = useState(includedItems);

  const containerRef = useRef<HTMLDivElement | null>(null);
  const [isInView, setIsInView] = useState(false);
  const hasFetchedRef = useRef(false);

  // Keep localIncludedItems in sync with prop
  useEffect(() => {
    setLocalIncludedItems(includedItems);
  }, [includedItems]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        setIsInView(entry.isIntersecting);
      },
      {
        root: null,
        // start fetch a bit before it appears
        rootMargin: "200px 0px",
        threshold: 0.1,
      }
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!isInView) return;
    if (hasFetchedRef.current) return;

    if (
      mediaItem &&
      mediaItem.type === "show" &&
      !mediaItem.series &&
      initialMediaKey &&
      localIncludedItems &&
      localIncludedItems[initialMediaKey]
    ) {
      hasFetchedRef.current = true;
      let cancelled = false;

      GetMediaItemDetails(mediaItem.title, mediaItem.rating_key, mediaItem.library_title, "item")
        .then((details) => {
          if (!cancelled && details.data?.media_item?.series) {
            setMediaItem((prev) => (prev ? { ...prev, series: details?.data?.media_item?.series } : prev));
            setLocalIncludedItems((prev) => {
              if (!prev) return prev;
              return {
                ...prev,
                [initialMediaKey]: {
                  ...prev[initialMediaKey],
                  media_item: {
                    ...prev[initialMediaKey].media_item,
                    series: details.data?.media_item?.series,
                  },
                },
              };
            });
          }
        })
        .catch(() => {
          // allow retry if needed
          hasFetchedRef.current = false;
        });

      return () => {
        cancelled = true;
      };
    }
  }, [isInView, mediaItem, initialMediaKey, localIncludedItems]);

  // If set or includedItems change, reset mediaItem
  useEffect(() => {
    setMediaItem(initialMediaItem);
  }, [set, includedItems, initialMediaItem]);

  return (
    <div ref={containerRef}>
      <MediaCarousel
        set={set}
        includedItems={localIncludedItems}
        mediaItem={mediaItem || ({} as MediaItem)}
        dimNotFound={true}
      />
    </div>
  );
};

export const RenderBoxsetDisplay = ({
  set,
  includedItems,
}: {
  set: CreatorBoxset;
  includedItems?: { [tmdb_id: string]: IncludedItem };
}) => {
  const [localIncludedItems, setLocalIncludedItems] = useState(includedItems);
  const [isOpen, setIsOpen] = useState(false);

  // Keep localIncludedItems in sync with prop
  useEffect(() => {
    setLocalIncludedItems(includedItems);
  }, [includedItems]);

  useEffect(() => {
    if (!isOpen || !localIncludedItems) return;

    // Collect only tmdb IDs that belong to this accordion's boxset
    const boxsetItemIds = new Set<string>();
    for (const posterSet of set.sets || []) {
      for (const itemId of posterSet.item_ids || []) {
        boxsetItemIds.add(itemId);
      }
    }

    // Only fetch show items in THIS accordion that are missing series
    const itemsToUpdate = Object.entries(localIncludedItems).filter(
      ([itemId, item]) =>
        boxsetItemIds.has(itemId) &&
        item.media_item &&
        item.media_item.type === "show" &&
        item.media_item.title &&
        item.media_item.rating_key &&
        !item.media_item.series
    );

    if (itemsToUpdate.length === 0) return;

    let cancelled = false;

    Promise.all(
      itemsToUpdate.map(async ([key, item]) => {
        try {
          const details = await GetMediaItemDetails(
            item.media_item.title,
            item.media_item.rating_key,
            item.media_item.library_title,
            "item"
          );
          return { key, series: details.data?.media_item?.series };
        } catch {
          return { key, series: undefined };
        }
      })
    ).then((results) => {
      if (cancelled) return;
      setLocalIncludedItems((prev) => {
        if (!prev) return prev;
        const updated = { ...prev };
        for (const { key, series } of results) {
          if (series) {
            updated[key] = {
              ...updated[key],
              media_item: {
                ...updated[key].media_item,
                series,
              },
            };
          }
        }
        return updated;
      });
    });

    return () => {
      cancelled = true;
    };
  }, [isOpen, localIncludedItems, set.sets]);

  return (
    <Accordion
      type="single"
      collapsible
      className="w-full"
      value={isOpen ? set.id : ""}
      onValueChange={(value) => setIsOpen(value === set.id)}
    >
      <AccordionItem value={set.id}>
        <AccordionTrigger className="flex items-center justify-between">
          <div className="text-primary-dynamic hover:text-primary cursor-pointer text-lg font-semibold">
            {set.title}
          </div>
        </AccordionTrigger>
        <AccordionContent>
          <div className="flex justify-end">
            <DownloadModal baseSetInfo={set} formItems={setRefsToFormItems(set.sets || [], localIncludedItems || {})} />
          </div>
          <ShowFullSetsDisplay
            baseSetInfo={set}
            posterSets={set.sets || []}
            includedItems={localIncludedItems}
            dimNotFound={true}
          />
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  );
};
