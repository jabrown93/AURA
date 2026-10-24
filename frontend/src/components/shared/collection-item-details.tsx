"use client";

import type { CollectionItem } from "@/app/collections/page";

import { AssetImage } from "@/components/shared/asset-image";
import { CollectionMediaItemsCarousel } from "@/components/shared/collection-media-items-carousel";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { H1, Lead } from "@/components/ui/typography";

type CollectionItemDetailsProps = {
  collectionItem: CollectionItem;
};

export function CollectionItemDetails({ collectionItem }: CollectionItemDetailsProps) {
  const minYear = Array.isArray(collectionItem.media_items)
    ? collectionItem.media_items.reduce((min, item) => (item.year < min ? item.year : min), Infinity)
    : undefined;
  const maxYear = Array.isArray(collectionItem.media_items)
    ? collectionItem.media_items.reduce((max, item) => (item.year > max ? item.year : max), -Infinity)
    : undefined;

  return (
    <div>
      <div className="flex flex-col lg:flex-row pt-30 items-center lg:items-start text-center lg:text-left">
        {/* Poster Image */}
        <div className="flex-shrink-0 mb-4 lg:mb-0 lg:mr-8 flex justify-center">
          <AssetImage
            image={`/api/images/media/collection?rating_key=${collectionItem?.rating_key}&image_type=poster&cb=${Date.now()}`}
            imageType="url"
            className="w-[200px] h-auto transition-transform hover:scale-105 select-none"
          />
        </div>

        {/* Title and Summary */}
        <div className="flex flex-col items-center lg:items-start">
          <H1 className="mb-1">{collectionItem?.title}</H1>
          {/* Hide summary on mobile */}
          <Lead className="text-primary-dynamic max-w-xl hidden lg:block">{collectionItem.summary}</Lead>
          {minYear !== undefined && maxYear !== undefined && (
            <Badge variant="default" className="mt-2 mb-2">
              {minYear === maxYear ? `${minYear}` : `${minYear} - ${maxYear}`}
            </Badge>
          )}
        </div>
      </div>

      {/* Library Information */}
      {collectionItem.library_title && (
        <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide mt-0 md:mt-2">
          <Lead className="text-md text-primary-dynamic ml-1">
            <span className="font-semibold">{collectionItem.library_title} Library</span>
          </Lead>
        </div>
      )}

      {/* Loop through the Media Items and display their posters */}
      {collectionItem.media_items && collectionItem.media_items.length > 0 && (
        <Accordion type="single" collapsible className="w-full">
          <AccordionItem value="media-items">
            <AccordionTrigger className="text-primary font-semibold">
              View {collectionItem.media_items.length} Media Items
            </AccordionTrigger>
            <AccordionContent>
              <CollectionMediaItemsCarousel mediaItems={collectionItem.media_items} />
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      )}
    </div>
  );
}
