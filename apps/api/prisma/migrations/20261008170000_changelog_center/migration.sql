CREATE TABLE "ChangelogEntry" (
    "id" TEXT NOT NULL,
    "version" VARCHAR(30),
    "title" VARCHAR(120) NOT NULL,
    "summary" VARCHAR(300),
    "releaseDate" TIMESTAMP(3) NOT NULL,
    "surfaces" TEXT[] DEFAULT ARRAY[]::TEXT[],
    "category" VARCHAR(24) NOT NULL DEFAULT 'IMPROVEMENT',
    "items" JSONB NOT NULL,
    "status" VARCHAR(16) NOT NULL DEFAULT 'DRAFT',
    "createdById" TEXT NOT NULL,
    "createdByName" VARCHAR(80) NOT NULL,
    "publishedAt" TIMESTAMP(3),
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "ChangelogEntry_pkey" PRIMARY KEY ("id")
);

CREATE INDEX "ChangelogEntry_status_releaseDate_idx" ON "ChangelogEntry"("status", "releaseDate");
CREATE INDEX "ChangelogEntry_category_releaseDate_idx" ON "ChangelogEntry"("category", "releaseDate");
